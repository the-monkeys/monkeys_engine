package database

import (
	"database/sql"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

func (uh *uDBHandler) AddPermissionToAUser(blogId string, userId int64, inviterID string, permissionType string) error {
	// Start a transaction to ensure data consistency
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction: %+v", err)
		return err
	}

	// Fetch the inviter ID based on inviter username
	var inviterId int64
	err = tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, inviterID).Scan(&inviterId)
	if err != nil {
		uh.log.Errorf("Failed to fetch inviter ID for user: %s, error: %+v", inviterID, err)
		tx.Rollback()
		return err
	}

	// Fetch the blog ID based on the blog_id string
	var blogIdInt int64
	err = tx.QueryRow(`SELECT id FROM blog WHERE blog_id = $1`, blogId).Scan(&blogIdInt)
	if err != nil {
		uh.log.Errorf("Failed to fetch blog ID for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
		return err
	}

	// Step 1: Check if the permission already exists in blog_permissions
	var exists int
	err = tx.QueryRow(`
		SELECT COUNT(1) 
		FROM blog_permissions 
		WHERE blog_id = $1 AND user_id = $2 AND permission_type = $3`,
		blogIdInt, userId, permissionType).Scan(&exists)
	if err != nil {
		uh.log.Errorf("Error checking permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	if exists == 0 {
		// Step 2: Add the permission to the blog_permissions table if it doesn't exist
		_, err = tx.Exec(`INSERT INTO blog_permissions (blog_id, user_id, permission_type) VALUES ($1, $2, $3)`,
			blogIdInt, userId, permissionType)
		if err != nil {
			uh.log.Errorf("Failed to add permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
			tx.Rollback()
			return err
		}
	}

	// Step 3: Check if the invite already exists in co_author_invites
	err = tx.QueryRow(`
		SELECT COUNT(1) 
		FROM co_author_invites 
		WHERE blog_id = $1 AND inviter_id = $2 AND invitee_id = $3`,
		blogIdInt, inviterId, userId).Scan(&exists)
	if err != nil {
		uh.log.Errorf("Error checking invite for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	if exists == 0 {
		// Step 4: Insert into co_author_invites if it doesn't exist
		_, err = tx.Exec(`INSERT INTO co_author_invites (blog_id, inviter_id, invitee_id, invite_status) VALUES ($1, $2, $3, 'pending')`,
			blogIdInt, inviterId, userId)
		if err != nil {
			uh.log.Errorf("Failed to invite co-author for blog: %s, user: %d, error: %+v", blogId, userId, err)
			tx.Rollback()
			return err
		}
	}

	// Step 5: Check if the user already has the permission in co_author_permissions
	err = tx.QueryRow(`
		SELECT COUNT(1) 
		FROM co_author_permissions 
		WHERE blog_id = $1 AND co_author_id = $2`,
		blogIdInt, userId).Scan(&exists)
	if err != nil {
		uh.log.Errorf("Error checking co-author permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	if exists == 0 {
		// Step 6: Fetch role ID based on the permissionType
		var roleId int64
		err = tx.QueryRow(`SELECT id FROM user_role WHERE role_desc = $1`, permissionType).Scan(&roleId)
		if err != nil {
			uh.log.Errorf("Failed to fetch role ID for permission type: %s, error: %+v", permissionType, err)
			tx.Rollback()
			return err
		}

		// Step 7: Insert into co_author_permissions if it doesn't exist
		_, err = tx.Exec(`INSERT INTO co_author_permissions (blog_id, co_author_id, role_id) VALUES ($1, $2, $3)`,
			blogIdInt, userId, roleId)
		if err != nil {
			uh.log.Errorf("Failed to grant co-author permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
			tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		uh.log.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	uh.log.Infof("Successfully added permission for blog: %s, user: %d", blogId, userId)
	return nil
}

func (uh *uDBHandler) RevokeBlogPermissionFromAUser(blogId string, userId int64, permissionType string) error {
	// Start a transaction
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction: %+v", err)
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

	// Step 2: Remove the permission from the blog_permissions table
	_, err = tx.Exec(`DELETE FROM blog_permissions WHERE blog_id = $1 AND user_id = $2 AND permission_type = $3`,
		blogIdInt, userId, permissionType)
	if err != nil {
		uh.log.Errorf("Failed to remove permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	// Step 3: Remove the user from the co_author_permissions table
	_, err = tx.Exec(`DELETE FROM co_author_permissions WHERE blog_id = $1 AND co_author_id = $2`,
		blogIdInt, userId)
	if err != nil {
		uh.log.Errorf("Failed to remove co-author permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	// Step 4: Update or remove the entry from co_author_invites
	_, err = tx.Exec(`DELETE FROM co_author_invites WHERE blog_id = $1 AND invitee_id = $2`,
		blogIdInt, userId)
	if err != nil {
		uh.log.Errorf("Failed to update co-author invites for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		uh.log.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	uh.log.Infof("Successfully revoked permission for blog: %s, user: %d", blogId, userId)
	return nil
}

func (uh *uDBHandler) GetBlogsByBlogId(blogId string) (models.Blog, error) {
	// Define the SQL query to retrieve the blog along with user details and permission
	query := `
        SELECT b.id, b.blog_id, u.username, u.account_id, bp.permission_type, b.status
        FROM blog b
        JOIN user_account u ON b.user_id = u.id
        LEFT JOIN blog_permissions bp ON b.id = bp.blog_id AND bp.user_id = u.id
        WHERE b.blog_id = $1
    `

	// Create a Blog variable to hold the result
	var blog models.Blog

	// Execute the query and scan the results into the blog struct
	err := uh.db.QueryRow(query, blogId).Scan(
		&blog.Id,
		&blog.BlogId,
		&blog.Username,
		&blog.AccountId,
		&blog.Permission,
		&blog.BlogStatus,
	)

	// Handle errors
	if err != nil {
		if err == sql.ErrNoRows {
			// Return a specific error if no blog is found
			uh.log.Warnf("No blog found with blogId: %s", blogId)
			return models.Blog{}, nil
		}
		uh.log.Errorf("Error retrieving blog with blogId %s: %+v", blogId, err)
		return models.Blog{}, err
	}

	return blog, nil
}
