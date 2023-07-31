package data

import (
	"context"
	"database/sql"
	"github.com/lib/pq"
	"time"
)

type PermissionsModelInterface interface {
	GetAllForUser(userID int64) (Permissions, error)
	AddForUser(userID int64, codes ...string) error
}

var _ PermissionsModelInterface = PermissionModel{}

type Permissions []string

func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true
		}
	}
	return false
}

type PermissionModel struct {
	DB *sql.DB
}

func (pm PermissionModel) AddForUser(userID int64, codes ...string) error {
	query := `
		insert into users_permissions
		select $1, permissions.id from permissions where permissions.code = any($2)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := pm.DB.ExecContext(ctx, query, userID, pq.Array(codes))
	return err
}

func (pm PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
	query := `
		select permissions.code
		from permissions p
		inner join users_permissions up on up.permission_id = p.id
		inner join users u on up.user_id = u.id
		where u.id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := pm.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission string

		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}
