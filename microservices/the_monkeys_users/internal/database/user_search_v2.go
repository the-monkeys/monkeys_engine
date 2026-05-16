package database

import (
	"context"
	"strings"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

// hardLimitV2 caps the page size accepted by FindUsersV2 so a single
// caller cannot exfiltrate the whole user table in one request. The
// gateway already enforces a softer cap; this is defence-in-depth.
const hardLimitV2 = 50

// FindUsersV2 is the search-v2 (Phase 1) implementation of people search.
//
// Improvements over FindUsersWithPagination:
//
//  1. Index-backed. Uses the pg_trgm GIN indexes added in migration
//  000007. Replaces the `ILIKE '%term%'` full-table-scan with two
//     index-friendly probes — exact/prefix on username and trigram
//     similarity on the combined search_doc column.
//  2. Ranked results. The CASE expression gives exact-username matches
//     the highest score, then prefix, then trigram similarity over the
//     combined doc. Output is ordered by rank desc so the "obvious" hit
//     is always first; ties break on alphabetical username for stable
//     pagination.
//  3. Active-only. Joins user_status and filters to 'Active'. Inactive
//     and hidden accounts no longer leak into search results.
//  4. Context-aware. Accepts a context.Context so the caller (gRPC
//     server) can propagate cancellation / deadline.
//  5. Hard-capped page size. Defends the DB even if the upstream forgets.
//
// The function returns models.UserAccount records (not protobuf) to keep
// the database layer independent of the wire format. Mapping to pb.User
// happens in the service layer.
func (uh *uDBHandler) FindUsersV2(ctx context.Context, searchTerm string, limit, offset int) ([]models.UserAccount, error) {
	term := strings.TrimSpace(searchTerm)
	if term == "" {
		// Empty searches would otherwise match every active user and burn
		// the trigram index. Fail fast instead.
		return nil, nil
	}

	if limit <= 0 || limit > hardLimitV2 {
		limit = hardLimitV2
	}
	if offset < 0 {
		offset = 0
	}

	// Notes on the query:
	//   * `<%` is the pg_trgm word_similarity operator. Unlike `%`
	//     (similarity), it scores the query against the BEST-MATCHING
	//     extent of the indexed text rather than the whole string, so
	//     a short query word like "joseph" still matches a long
	//     search_doc that includes "Shinu Joseph ...long bio...".
	//     Verified empirically: similarity('joseph', 'SJ Shinu Joseph
	//     The fourth monkey') = 0.21 (below 0.3 threshold), whereas
	//     word_similarity is 1.0. The GIN(gin_trgm_ops) index on
	//     search_doc supports `<%` natively in PG 11+.
	//   * `username ILIKE $1 || '%'` (NOT leading-%) is index-friendly
	//     via the trigram GIN index on username.
	//   * lower() comparisons keep the CASE deterministic across locales.
	//   * word_similarity() is computed once per row, then re-used in
	//     ORDER BY so ranking matches the predicate that selected the
	//     row.
	//   * account_id is selected so callers (gRPC service) can resolve
	//     profile pic URIs deterministically.
	const query = `
		SELECT ua.account_id,
		       ua.username,
		       ua.first_name,
		       ua.last_name,
		       ua.bio,
		       ua.avatar_url,
		       (
		           CASE
		               WHEN lower(ua.username) = lower($1)              THEN 100
		               WHEN lower(ua.username) LIKE lower($1) || '%'    THEN 50
		               ELSE 0
		           END
		           + COALESCE(word_similarity($1, ua.search_doc), 0) * 30
		       ) AS rank
		FROM   user_account ua
		JOIN   user_status  us ON us.id = ua.user_status
		WHERE  us.status = 'Active'
		  AND  ( $1 <% ua.search_doc
		         OR ua.username ILIKE $1 || '%' )
		ORDER  BY rank DESC, ua.username ASC
		LIMIT  $2 OFFSET $3;
	`

	rows, err := uh.db.QueryContext(ctx, query, term, limit, offset)
	if err != nil {
		uh.log.Errorf("v2 user search failed: term=%q err=%v", term, err)
		return nil, err
	}
	defer func() {
		if cErr := rows.Close(); cErr != nil {
			uh.log.Warnf("closing v2 user search rows: %v", cErr)
		}
	}()

	// Pre-size the slice to the cap to avoid the first few grow steps.
	users := make([]models.UserAccount, 0, limit)
	for rows.Next() {
		var (
			u    models.UserAccount
			rank float64 // computed by the DB; not surfaced upstream today
		)
		if err := rows.Scan(
			&u.AccountId,
			&u.UserName,
			&u.FirstName,
			&u.LastName,
			&u.Bio,
			&u.AvatarUrl,
			&rank,
		); err != nil {
			uh.log.Errorf("scan v2 user search row: %v", err)
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		uh.log.Errorf("iterate v2 user search rows: %v", err)
		return nil, err
	}

	return users, nil
}
