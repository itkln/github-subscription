package subscription

import (
	"context"
	"database/sql"

	subscriptionmodel "github.com/itkln/github-subscription/internal/model/subscription"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, params subscriptionmodel.CreateParams) (subscriptionmodel.DBSubscription, error) {
	const query = `
		INSERT INTO subscriptions (email, repo, confirm_token, unsubscribe_token)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
	`

	var subscription subscriptionmodel.DBSubscription
	err := r.db.QueryRowContext(
		ctx,
		query,
		params.Email,
		params.Repo,
		params.ConfirmToken,
		params.UnsubscribeToken,
	).Scan(
		&subscription.ID,
		&subscription.Email,
		&subscription.Repo,
		&subscription.Confirmed,
		&subscription.ConfirmToken,
		&subscription.UnsubscribeToken,
		&subscription.LastSeenTag,
	)
	if err != nil {
		return subscriptionmodel.DBSubscription{}, err
	}

	return subscription, nil
}

func (r *Repository) ExistsByEmailAndRepo(ctx context.Context, email, repo string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM subscriptions
			WHERE email = $1 AND repo = $2
		)
	`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, email, repo).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (r *Repository) GetByConfirmToken(ctx context.Context, token string) (subscriptionmodel.DBSubscription, error) {
	const query = `	
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions
		WHERE confirm_token = $1
	`

	return r.getOne(ctx, query, token)
}

func (r *Repository) GetByUnsubscribeToken(ctx context.Context, token string) (subscriptionmodel.DBSubscription, error) {
	const query = `
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions
		WHERE unsubscribe_token = $1
	`

	return r.getOne(ctx, query, token)
}

func (r *Repository) ConfirmByToken(ctx context.Context, token string) error {
	const query = `
		UPDATE subscriptions
		SET confirmed = TRUE,
		    updated_at = NOW()
		WHERE confirm_token = $1
	`

	result, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *Repository) DeleteByUnsubscribeToken(ctx context.Context, token string) error {
	const query = `
		DELETE FROM subscriptions
		WHERE unsubscribe_token = $1
	`

	result, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *Repository) ListActiveByEmail(ctx context.Context, email string) (_ []subscriptionmodel.DBSubscription, err error) {
	const query = `
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions
		WHERE email = $1 AND confirmed = TRUE
		ORDER BY repo ASC
	`

	rows, err := r.db.QueryContext(ctx, query, email)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := rows.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	subscriptions := make([]subscriptionmodel.DBSubscription, 0)
	for rows.Next() {
		var subscription subscriptionmodel.DBSubscription
		if err := rows.Scan(
			&subscription.ID,
			&subscription.Email,
			&subscription.Repo,
			&subscription.Confirmed,
			&subscription.ConfirmToken,
			&subscription.UnsubscribeToken,
			&subscription.LastSeenTag,
		); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (r *Repository) ListConfirmed(ctx context.Context) (_ []subscriptionmodel.DBSubscription, err error) {
	const query = `
		SELECT id, email, repo, confirmed, confirm_token, unsubscribe_token, last_seen_tag
		FROM subscriptions
		WHERE confirmed = TRUE
		ORDER BY repo ASC, email ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := rows.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	subscriptions := make([]subscriptionmodel.DBSubscription, 0)
	for rows.Next() {
		var subscription subscriptionmodel.DBSubscription
		if err := rows.Scan(
			&subscription.ID,
			&subscription.Email,
			&subscription.Repo,
			&subscription.Confirmed,
			&subscription.ConfirmToken,
			&subscription.UnsubscribeToken,
			&subscription.LastSeenTag,
		); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (r *Repository) UpdateLastSeenTag(ctx context.Context, id int64, tag string) error {
	const query = `
		UPDATE subscriptions
		SET last_seen_tag = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, tag)
	if err != nil {
		return err
	}

	return ensureRowsAffected(result)
}

func (r *Repository) getOne(ctx context.Context, query, token string) (subscriptionmodel.DBSubscription, error) {
	var subscription subscriptionmodel.DBSubscription
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&subscription.ID,
		&subscription.Email,
		&subscription.Repo,
		&subscription.Confirmed,
		&subscription.ConfirmToken,
		&subscription.UnsubscribeToken,
		&subscription.LastSeenTag,
	)
	if err != nil {
		return subscriptionmodel.DBSubscription{}, err
	}

	return subscription, nil
}

func ensureRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
