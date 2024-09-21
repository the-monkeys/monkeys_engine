package database

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
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
