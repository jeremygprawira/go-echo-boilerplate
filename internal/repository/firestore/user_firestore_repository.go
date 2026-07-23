package firestore

import (
	"context"

	"go-echo-boilerplate/internal/models"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserRepository is a Firestore-backed adapter satisfying repository.UserRepository.
// Documents are keyed by AccountNumber in the "users" collection.
type UserRepository struct {
	col *fs.CollectionRef
}

func NewUserRepository(client *fs.Client) *UserRepository {
	return &UserRepository{col: client.Collection("users")}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	_, err := r.col.Doc(user.AccountNumber).Set(ctx, user)
	return err
}

func (r *UserRepository) CheckByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (bool, error) {
	if email != "" {
		if user, err := r.queryOne(ctx, "email", email); err != nil {
			return false, err
		} else if user != nil {
			return true, nil
		}
	}
	if phoneNumber != "" {
		if user, err := r.queryOne(ctx, "phone_number", phoneNumber); err != nil {
			return false, err
		} else if user != nil {
			return true, nil
		}
	}
	return false, nil
}

func (r *UserRepository) GetCredentialsByEmailOrPhoneNumber(ctx context.Context, email string, phoneNumber string) (*models.User, error) {
	if email != "" {
		user, err := r.queryOne(ctx, "email", email)
		if err != nil {
			return nil, err
		}
		if user != nil {
			return user, nil
		}
	}
	if phoneNumber != "" {
		return r.queryOne(ctx, "phone_number", phoneNumber)
	}
	return nil, nil
}

func (r *UserRepository) GetOneByAccountNumber(ctx context.Context, accountNumber string) (*models.User, error) {
	doc, err := r.col.Doc(accountNumber).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var u models.User
	if err := doc.DataTo(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetOneByID(ctx context.Context, id int) (*models.User, error) {
	return r.queryOne(ctx, "id", id)
}

// queryOne runs a single-field equality query and returns the first match, or
// (nil, nil) when no document matches — matching the pgsql adapter's
// not-found contract used by userService.
func (r *UserRepository) queryOne(ctx context.Context, field string, value interface{}) (*models.User, error) {
	iter := r.col.Where(field, "==", value).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var u models.User
	if err := doc.DataTo(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
