package database

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

func (userDB *uDBHandler) CreateUserLog(user *models.UserLogs, description string) error {
	var userId int64
	var clientId int8
	var err error

	//From username find user id
	if err = userDB.db.QueryRow(`
			SELECT id FROM user_account WHERE account_id = $1;`, user.AccountId).Scan(&userId); err != nil {
		logrus.Errorf("can't get id by using account_id %s, error: %v", user.AccountId, err)
		return err
	}

	//From client name find client id
	if err := userDB.db.QueryRow(`
			SELECT id FROM clients WHERE c_name = $1;`, user.Client).Scan(&clientId); err != nil {
		logrus.Errorf("can't get id by using client name %s, error: %+v", user.Client, err)
		return err
	}

	stmt, err := userDB.db.Prepare(`INSERT INTO user_account_log (user_id, ip_address, description, client_id) VALUES ($1, $2, $3, $4);`)
	if err != nil {
		logrus.Errorf("cannot prepare statement to add user activity into the user_account_log: %v", err)
		return err
	}
	defer stmt.Close()

	row, err := stmt.Exec(userId, user.IpAddress, description, clientId)
	if err != nil {
		logrus.Errorf("cannot execute query to add user to the user_account_log: %v", err)
		return err
	}

	affectedRow, err := row.RowsAffected()
	if err != nil {
		logrus.Errorf("error finding affected rows for user_account_log: %v", err)
		return err
	}

	if affectedRow == 0 {
		logrus.Errorf("cannot create a record in the log table for user_account_log: %v", err)
		return errors.New("cannot create a record in the log table")
	}

	return nil
}

// GetBlogsByUserName fetches blogs by username with permission and blog status
func (uh *uDBHandler) GetBlogsByUserName(username string) (*pb.BlogsByUserNameRes, error) {
	// Step 1: Prepare the query
	query := `
		SELECT b.id, b.blog_id, ua.username, ua.account_id, bp.permission_type, b.status
		FROM blog b
		JOIN blog_permissions bp ON b.id = bp.blog_id
		JOIN user_account ua ON bp.user_id = ua.id
		WHERE ua.username = $1;
	`

	// Step 2: Execute the query
	rows, err := uh.db.Query(query, username)
	if err != nil {
		uh.log.Errorf("Error fetching blogs for username %s, error: %+v", username, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the results into a slice of Blog structs
	var blogs []*pb.Blog
	for rows.Next() {
		var blog models.Blog
		err := rows.Scan(&blog.Id, &blog.BlogId, &blog.Username, &blog.AccountId, &blog.Permission, &blog.BlogStatus)
		if err != nil {
			uh.log.Errorf("Error scanning blog data for username %s, error: %+v", username, err)
			return nil, err
		}
		pbBlog := &pb.Blog{
			Id:         blog.Id,
			BlogId:     blog.BlogId,
			Username:   blog.Username,
			AccountId:  blog.AccountId,
			Permission: blog.Permission,
			Status:     blog.BlogStatus,
		}
		blogs = append(blogs, pbBlog)
	}

	// Step 4: Check for errors after iterating over the rows
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching blogs for username %s, error: %+v", username, err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched blogs for user: %s", username)
	return &pb.BlogsByUserNameRes{
		Blogs: blogs,
	}, nil
}

// GetBlogsByUserIdWithEditorAccess fetches blogs by user account ID where the user has Editor access
func (uh *uDBHandler) GetBlogsByUserIdWithEditorAccess(accountId int64) (*pb.BlogsByUserNameRes, error) {
	// Step 1: Prepare the query
	query := `
		SELECT b.id, b.blog_id, ua.username, ua.account_id, bp.permission_type, b.status
		FROM blog b
		JOIN blog_permissions bp ON b.id = bp.blog_id
		JOIN user_account ua ON bp.user_id = ua.id
		WHERE ua.id = $1 AND bp.permission_type = 'Editor';
	`

	// Step 2: Execute the query
	rows, err := uh.db.Query(query, accountId)
	if err != nil {
		uh.log.Errorf("Error fetching blogs for user account ID %d, error: %+v", accountId, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the results into a slice of Blog structs
	var blogs []*pb.Blog
	for rows.Next() {
		var blog models.Blog
		err := rows.Scan(&blog.Id, &blog.BlogId, &blog.Username, &blog.AccountId, &blog.Permission, &blog.BlogStatus)
		if err != nil {
			uh.log.Errorf("Error scanning blog data for user account ID %d, error: %+v", accountId, err)
			return nil, err
		}
		pbBlog := &pb.Blog{
			Id:         blog.Id,
			BlogId:     blog.BlogId,
			Username:   blog.Username,
			AccountId:  blog.AccountId,
			Permission: blog.Permission,
			Status:     blog.BlogStatus,
		}
		blogs = append(blogs, pbBlog)
	}

	// Step 4: Check for errors after iterating over the rows
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching blogs for user account ID %d, error: %+v", accountId, err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched blogs with Editor access for user account ID: %d", accountId)
	return &pb.BlogsByUserNameRes{
		Blogs: blogs,
	}, nil
}

// GetBlogsByUserName fetches blogs by username with permission and blog status
func (uh *uDBHandler) GetBlogsByAccountId(accountId string) (*pb.BlogsByUserNameRes, error) {
	// Step 1: Prepare the query
	query := `
		SELECT b.id, b.blog_id, ua.username, ua.account_id, bp.permission_type, b.status
		FROM blog b
		JOIN blog_permissions bp ON b.id = bp.blog_id
		JOIN user_account ua ON bp.user_id = ua.id
		WHERE ua.account_id = $1;
	`

	// Step 2: Execute the query
	rows, err := uh.db.Query(query, accountId)
	if err != nil {
		uh.log.Errorf("Error fetching blogs for username %s, error: %+v", accountId, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the results into a slice of Blog structs
	var blogs []*pb.Blog
	for rows.Next() {
		var blog models.Blog
		err := rows.Scan(&blog.Id, &blog.BlogId, &blog.Username, &blog.AccountId, &blog.Permission, &blog.BlogStatus)
		if err != nil {
			uh.log.Errorf("Error scanning blog data for username %s, error: %+v", accountId, err)
			return nil, err
		}
		pbBlog := &pb.Blog{
			Id:         blog.Id,
			BlogId:     blog.BlogId,
			Username:   blog.Username,
			AccountId:  blog.AccountId,
			Permission: blog.Permission,
			Status:     blog.BlogStatus,
		}
		blogs = append(blogs, pbBlog)
	}

	// Step 4: Check for errors after iterating over the rows
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching blogs for username %s, error: %+v", accountId, err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched blogs for user: %s", accountId)
	return &pb.BlogsByUserNameRes{
		Blogs: blogs,
	}, nil
}

func (uh *uDBHandler) CreateNewTopics(topics []string, category, username string) error {
	// Start a transaction
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction: %+v", err)
		return err
	}

	// Step 1: Fetch the user ID based on username
	var userId int64
	err = tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userId)
	if err != nil {
		uh.log.Errorf("Failed to fetch user ID for username: %s, error: %+v", username, err)
		tx.Rollback() // rollback transaction on error
		return err
	}

	// Step 2: Iterate over the interests and insert them into the user_interest table
	for _, topic := range topics {
		// Check if the user is already following this interest
		var exists int
		err = tx.QueryRow(`SELECT COUNT(1) FROM topics WHERE description = $1`, topic).Scan(&exists)
		if err != nil {
			uh.log.Errorf("Failed to check if the topic already exists: %s, error: %+v", topic, err)
			tx.Rollback() // rollback transaction on error
			return err
		}

		// If the user is already following the interest, skip the insert and log it
		if exists > 0 {
			uh.log.Infof("Topic %s already exists", topic)
			continue
		}

		// Insert into user_interest table for interests not already followed
		_, err = tx.Exec(`INSERT INTO topics (description, category, user_id) VALUES ($1, $2, $3)`, topic, category, userId)
		if err != nil {
			uh.log.Errorf("Failed to insert topic %s for username: %s, error: %+v", topic, username, err)
			tx.Rollback() // rollback transaction on error
			return err
		}
	}

	// Step 4: Commit the transaction
	if err := tx.Commit(); err != nil {
		uh.log.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	uh.log.Infof("Successfully added new interests for user: %s", username)
	return nil
}

func (uh *uDBHandler) GetCoAuthorBlogsByAccountId(accountId string) (*pb.BlogsByUserNameRes, error) {
	// Step 1: Prepare the query
	query := `
	SELECT b.id, b.blog_id, ua.username, ua.account_id, bp.permission_type, b.status
	FROM blog b
	JOIN blog_permissions bp ON b.id = bp.blog_id
	JOIN user_account ua ON bp.user_id = ua.id
	WHERE ua.account_id = $1 AND bp.permission_type = $2;
	`

	// Step 2: Execute the query
	rows, err := uh.db.Query(query, accountId, constants.RoleEditor)
	if err != nil {
		uh.log.Errorf("Error fetching blogs for username %s, error: %+v", accountId, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the results into a slice of Blog structs
	var blogs []*pb.Blog
	for rows.Next() {
		var blog models.Blog
		err := rows.Scan(&blog.Id, &blog.BlogId, &blog.Username, &blog.AccountId, &blog.Permission, &blog.BlogStatus)
		if err != nil {
			uh.log.Errorf("Error scanning blog data for username %s, error: %+v", accountId, err)
			return nil, err
		}
		pbBlog := &pb.Blog{
			Id:         blog.Id,
			BlogId:     blog.BlogId,
			Username:   blog.Username,
			AccountId:  blog.AccountId,
			Permission: blog.Permission,
			Status:     blog.BlogStatus,
		}
		blogs = append(blogs, pbBlog)
	}

	// Step 4: Check for errors after iterating over the rows
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching blogs for username %s, error: %+v", accountId, err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched blogs for user: %s", accountId)
	return &pb.BlogsByUserNameRes{
		Blogs: blogs,
	}, nil
}

func (uh *uDBHandler) BookMarkABlog(blogId string, userId int64) error {
	blogIdInt := 0
	err := uh.db.QueryRow(`SELECT id FROM blog WHERE blog_id = $1`, blogId).Scan(&blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to fetch blog ID for blogId: %s, error: %+v", blogId, err)
		return err
	}

	var exists int
	err = uh.db.QueryRow(`SELECT COUNT(1) FROM blog_bookmarks WHERE user_id = $1 AND blog_id = $2`, userId, blogIdInt).Scan(&exists)
	if err != nil {
		uh.log.Errorf("Failed to check if the bookmark already exists: %s, error: %+v", blogId, err)
		return err
	}

	if exists > 0 {
		uh.log.Infof("bookmark already exists")
		return nil
	}

	query := `
		INSERT INTO blog_bookmarks (user_id, blog_id) VALUES ($1, $2);
	`

	_, err = uh.db.Exec(query, userId, blogIdInt)
	if err != nil {
		uh.log.Errorf("Error bookmarking blog %s for user %d, error: %+v", blogId, userId, err)
		return err
	}

	uh.log.Infof("Successfully bookmarked blog %s for user %d", blogId, userId)
	return nil
}

func (uh *uDBHandler) RemoveBookmarkFromBlog(blogId string, userId int64) error {
	blogIdInt := 0
	err := uh.db.QueryRow(`SELECT id FROM blog WHERE blog_id = $1`, blogId).Scan(&blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to fetch blog ID for blogId: %s, error: %+v", blogId, err)
		return err
	}

	query := `
		DELETE FROM blog_bookmarks WHERE user_id = $1 AND blog_id = $2;
	`

	_, err = uh.db.Exec(query, userId, blogIdInt)
	if err != nil {
		uh.log.Errorf("Error removing bookmark from blog %s for user %d, error: %+v", blogId, userId, err)
		return err
	}

	uh.log.Infof("Successfully removed bookmark from blog %s for user %d", blogId, userId)
	return nil
}

// -- Creating blog bookmarks table
// CREATE TABLE IF NOT EXISTS blog_bookmarks (
//
//	id SERIAL PRIMARY KEY,
//	user_id BIGINT NOT NULL,
//	blog_id BIGINT NOT NULL,
//	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//	FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
//	FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION
//
// );
func (uh *uDBHandler) GetBookmarkBlogsByAccountId(accountId string) (*pb.BlogsByUserNameRes, error) {
	// Step 1: Prepare the query
	query := `
	SELECT b.id, b.blog_id, ua.username
	FROM blog b
	JOIN blog_bookmarks bb ON b.id = bb.blog_id  -- Use 'b' here instead of 'blog'
	JOIN user_account ua ON bb.user_id = ua.id
	WHERE ua.account_id = $1;
	`

	// Step 2: Execute the query
	rows, err := uh.db.Query(query, accountId)
	if err != nil {
		uh.log.Errorf("Error fetching blogs for username %s, error: %+v", accountId, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the results into a slice of Blog structs
	var blogs []*pb.Blog
	for rows.Next() {
		var blog models.Blog
		err := rows.Scan(&blog.Id, &blog.BlogId, &blog.Username)
		if err != nil {
			uh.log.Errorf("Error scanning blog data for username %s, error: %+v", accountId, err)
			return nil, err
		}
		pbBlog := &pb.Blog{
			Id:       blog.Id,
			BlogId:   blog.BlogId,
			Username: blog.Username,
		}
		blogs = append(blogs, pbBlog)
	}

	// Step 4: Check for errors after iterating over the rows
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching blogs for username %s, error: %+v", accountId, err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched blogs for user: %s", accountId)
	return &pb.BlogsByUserNameRes{
		Blogs: blogs,
	}, nil

}

// DeleteBlogAndReferences deletes a blog and all its references from related tables
func (uh *uDBHandler) DeleteBlogAndReferences(blogId string) error {
	// Start a transaction to ensure data consistency
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction for deleting blog: %s, error: %+v", blogId, err)
		return err
	}

	// Step 1: Fetch the blog ID based on the blog_id string
	var blogIdInt int64
	err = tx.QueryRow(`SELECT id FROM blog WHERE blog_id = $1`, blogId).Scan(&blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to fetch blog ID for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Step 2: Delete from blog_permissions
	_, err = tx.Exec(`DELETE FROM blog_permissions WHERE blog_id = $1`, blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to delete from blog_permissions for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Step 3: Delete from co_author_invites
	_, err = tx.Exec(`DELETE FROM co_author_invites WHERE blog_id = $1`, blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to delete from co_author_invites for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Step 4: Delete from co_author_permissions
	_, err = tx.Exec(`DELETE FROM co_author_permissions WHERE blog_id = $1`, blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to delete from co_author_permissions for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Step 5: Delete from blog_bookmarks (if applicable)
	_, err = tx.Exec(`DELETE FROM blog_bookmarks WHERE blog_id = $1`, blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to delete from blog_bookmarks for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Step 6: Finally, delete the blog from the blog table
	_, err = tx.Exec(`DELETE FROM blog WHERE id = $1`, blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to delete blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		uh.log.Errorf("Failed to commit transaction for deleting blog: %s, error: %+v", blogId, err)
		return err
	}

	uh.log.Infof("Successfully deleted blog: %s and its references", blogId)
	return nil
}

func (uh *uDBHandler) LikeBlog(username string, blogExtId string) error {
	tx, err := uh.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var blogID int64
	// Step 1: Fetch the user ID using the username
	if err := tx.QueryRow(`SELECT id FROM blog WHERE blog_id = $1`, blogExtId).Scan(&blogID); err != nil {
		logrus.Errorf("Can't get ID for blog id %s, error: %+v", blogExtId, err)
		return err
	}

	var userID int64
	// Step 1: Fetch the user ID using the username
	if err := tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userID); err != nil {
		logrus.Errorf("Can't get ID for username %s, error: %+v", username, err)
		return err
	}

	// Step 2: Check if the like relationship already exists
	var exists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM blog_likes WHERE user_id = $1 AND blog_id = $2)`, userID, blogID).Scan(&exists)
	if err != nil {
		logrus.Errorf("Failed to check like relationship between user ID %d and blog ID %d, error: %+v", userID, blogID, err)
		return err
	}

	if exists {
		logrus.Infof("User %s has already liked blog ID %d", username, blogID)
		return nil
	}

	// Step 3: Insert like relationship
	_, err = tx.Exec(`INSERT INTO blog_likes (user_id, blog_id, created_at) VALUES ($1, $2, CURRENT_TIMESTAMP)`, userID, blogID)
	if err != nil {
		logrus.Errorf("Failed to insert like relationship between user ID %d and blog ID %d, error: %+v", userID, blogID, err)
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		logrus.Errorf("Failed to commit transaction for liking blog ID %d by user %s, error: %+v", blogID, username, err)
		return err
	}

	logrus.Infof("Successfully liked blog ID %d by user: %s", blogID, username)
	return nil
}

func (uh *uDBHandler) UnlikeBlog(username string, blogExtId string) error {
	tx, err := uh.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var blogID int64
	// Step 1: Fetch the user ID using the username
	if err := tx.QueryRow(`SELECT id FROM blog WHERE blog_id = $1`, blogExtId).Scan(&blogID); err != nil {
		logrus.Errorf("Can't get ID for blog id %s, error: %+v", username, err)
		return err
	}

	var userID int64
	// Step 1: Fetch the user ID using the username
	if err := tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userID); err != nil {
		logrus.Errorf("Can't get ID for username %s, error: %+v", username, err)
		return err
	}

	// Step 2: Check if the like relationship exists
	var exists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM blog_likes WHERE user_id = $1 AND blog_id = $2)`, userID, blogID).Scan(&exists)
	if err != nil {
		logrus.Errorf("Failed to check like relationship between user ID %d and blog ID %d, error: %+v", userID, blogID, err)
		return err
	}

	if !exists {
		logrus.Infof("User %s has not liked blog ID %d", username, blogID)
		return nil
	}

	// Step 3: Delete like relationship
	_, err = tx.Exec(`DELETE FROM blog_likes WHERE user_id = $1 AND blog_id = $2`, userID, blogID)
	if err != nil {
		logrus.Errorf("Failed to delete like relationship between user ID %d and blog ID %d, error: %+v", userID, blogID, err)
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		logrus.Errorf("Failed to commit transaction for unliking blog ID %d by user %s, error: %+v", blogID, username, err)
		return err
	}

	logrus.Infof("Successfully unliked blog ID %d by user: %s", blogID, username)
	return nil
}

func (uh *uDBHandler) IsUserFollowing(followerUsername string, followingUsername string) (bool, error) {
	query := `
        SELECT COUNT(1)
        FROM user_follows uf
        JOIN user_account u1 ON uf.follower_id = u1.id
        JOIN user_account u2 ON uf.following_id = u2.id
        WHERE u1.username = $1 AND u2.username = $2
    `

	var count int
	err := uh.db.QueryRow(query, followerUsername, followingUsername).Scan(&count)
	if err != nil {
		uh.log.Errorf("Error checking if %s follows %s: %+v", followerUsername, followingUsername, err)
		return false, err
	}

	return count > 0, nil
}
