package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/domain/entities"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *entities.ValidatedUser) (*entities.User, error) {
	_, err := r.db.Exec(
		`INSERT INTO users (id, email, password_hash, display_name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		user.Id.String(), user.Email, user.PasswordHash, user.DisplayName,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	result := user.User
	return &result, nil
}

func (r *UserRepository) FindById(id uuid.UUID) (*entities.User, error) {
	return r.scanUser(r.db.QueryRow(
		`SELECT id, email, password_hash, display_name, created_at, updated_at
		 FROM users WHERE id = ?`, id.String(),
	))
}

func (r *UserRepository) FindByEmail(email string) (*entities.User, error) {
	return r.scanUser(r.db.QueryRow(
		`SELECT id, email, password_hash, display_name, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	))
}

func (r *UserRepository) Update(user *entities.ValidatedUser) (*entities.User, error) {
	_, err := r.db.Exec(
		`UPDATE users SET email = ?, password_hash = ?, display_name = ?, updated_at = ?
		 WHERE id = ?`,
		user.Email, user.PasswordHash, user.DisplayName, user.UpdatedAt, user.Id.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}
	result := user.User
	return &result, nil
}

func (r *UserRepository) scanUser(row *sql.Row) (*entities.User, error) {
	var user entities.User
	var idStr string
	err := row.Scan(&idStr, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	user.Id, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parsing user id: %w", err)
	}
	return &user, nil
}
