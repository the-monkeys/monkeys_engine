package database

func (uh *uDBHandler) AddPermissionToAUser(blogId string, userId int64, inviterID string, permissionType string) error {
	// Start a transaction to ensure data consistency
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction: %+v", err)
		return err
	}

	var inviterId int64
	err = tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, inviterID).Scan(&inviterId)
	if err != nil {
		uh.log.Errorf("Failed to fetch blog ID for blog: %s, error: %+v", blogId, err)
		tx.Rollback()
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

	// Step 2: Add the permission to the blog_permissions table
	_, err = tx.Exec(`INSERT INTO blog_permissions (blog_id, user_id, permission_type) VALUES ($1, $2, $3)`,
		blogIdInt, userId, permissionType)
	if err != nil {
		uh.log.Errorf("Failed to add permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	// Step 3: Insert into co_author_invites (assuming permissionType involves co-authorship)
	_, err = tx.Exec(`INSERT INTO co_author_invites (blog_id, inviter_id, invitee_id, invite_status) VALUES ($1, $2, $3, 'pending')`,
		blogIdInt, inviterId, userId)
	if err != nil {
		uh.log.Errorf("Failed to invite co-author for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
	}

	// Step 4: Add the user to the co_author_permissions table with the corresponding role
	var roleId int64
	err = tx.QueryRow(`SELECT id FROM user_role WHERE role_desc = $1`, permissionType).Scan(&roleId)
	if err != nil {
		uh.log.Errorf("Failed to fetch role ID for permission type: %s, error: %+v", permissionType, err)
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`INSERT INTO co_author_permissions (blog_id, co_author_id, role_id) VALUES ($1, $2, $3)`,
		blogIdInt, userId, roleId)
	if err != nil {
		uh.log.Errorf("Failed to grant co-author permission for blog: %s, user: %d, error: %+v", blogId, userId, err)
		tx.Rollback()
		return err
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
