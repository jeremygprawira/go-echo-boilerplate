package pgsql

import (
	"context"
	"fmt"
	"go-echo-boilerplate/internal/pkg/logger"

	"gorm.io/gorm"
)

// TransactionRepository provides methods for managing database transactions.
// The Atomic method is the recommended approach for most use cases as it handles
// all transaction lifecycle management automatically.
type TransactionRepository interface {
	// Begin starts a new database transaction.
	// Note: Prefer using Atomic() for automatic transaction management.
	Begin(ctx context.Context) (*gorm.DB, error)

	// Commit commits the current transaction.
	// Note: This should only be used with manually managed transactions from Begin().
	Commit(ctx context.Context, tx *gorm.DB) error

	// Rollback rolls back the current transaction.
	// Note: This should only be used with manually managed transactions from Begin().
	Rollback(ctx context.Context, tx *gorm.DB) error

	// Atomic executes a function within a database transaction.
	// If the function returns an error or panics, the transaction is rolled back.
	// Otherwise, the transaction is committed.
	Atomic(ctx context.Context, fc func(ctx context.Context, r *PostgreRepository) error) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Begin starts a new database transaction with context support.
// Returns the transaction instance or an error if the transaction cannot be started.
func (tr *transactionRepository) Begin(ctx context.Context) (*gorm.DB, error) {
	tx := tr.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	return tx, nil
}

// Commit commits the provided transaction.
// The caller is responsible for passing the correct transaction instance.
func (tr *transactionRepository) Commit(ctx context.Context, tx *gorm.DB) error {
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the provided transaction.
// The caller is responsible for passing the correct transaction instance.
func (tr *transactionRepository) Rollback(ctx context.Context, tx *gorm.DB) error {
	if err := tx.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// Atomic executes the provided function within a database transaction.
// This is the recommended way to handle transactions as it ensures proper cleanup.
//
// Transaction Lifecycle:
// 1. Begin transaction
// 2. Execute the provided function
// 3. If function succeeds: Commit
// 4. If function fails or panics: Rollback
//
// Example usage:
//
//	err := repo.Transaction.Atomic(ctx, func(ctx context.Context, r *PostgreRepository) error {
//	    // Perform database operations using r
//	    if err := r.SomeRepo.Create(ctx, data); err != nil {
//	        return err // This will trigger a rollback
//	    }
//	    return nil // This will trigger a commit
//	})
func (tr *transactionRepository) Atomic(ctx context.Context, fc func(ctx context.Context, r *PostgreRepository) error) error {
	// Step 1: Begin the transaction with context support
	// WithContext ensures the transaction respects context cancellation
	tx := tr.db.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Step 2: Set up deferred cleanup to ensure transaction is always finalized
	// This runs after the function completes, regardless of success or failure
	defer func() {
		// Step 2a: Handle panic recovery
		// If a panic occurs during the transaction, we catch it here
		if p := recover(); p != nil {
			// Rollback the transaction due to panic
			tx.Rollback()
			// Convert panic to error for consistent error handling
			err = fmt.Errorf("transaction failed due to panic: %v", p)

			// Log the panic with rich error context
			// Note: The updated context from LogError won't propagate from defer,
			// but the middleware will still pick it up from the original context
			logger.LogError(ctx, logger.ErrorContext{
				Type:      "PanicError",
				Code:      "DATABASE_TRANSACTION_PANIC",
				Message:   err.Error(),
				Retriable: false,
			})

		} else if err != nil {
			// Step 2b: Handle explicit errors returned by the function
			// If the function returned an error, rollback the transaction
			if rbErr := tx.Rollback().Error; rbErr != nil {
				// If rollback itself fails, wrap both errors for debugging
				err = fmt.Errorf("failed to rollback transaction (original error: %w): %v", err, rbErr)

				logger.LogError(ctx, logger.ErrorContext{
					Type:      "RollbackError",
					Code:      "DATABASE_TRANSACTION_ROLLBACK",
					Message:   err.Error(),
					Retriable: false,
				})
			}
		} else {
			// Step 2c: Success path - commit the transaction
			// If no error occurred, commit all changes
			if commitErr := tx.Commit().Error; commitErr != nil {
				// If commit fails, update err so it's returned to caller
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)

				logger.LogError(ctx, logger.ErrorContext{
					Type:      "CommitError",
					Code:      "DATABASE_TRANSACTION_COMMIT",
					Message:   err.Error(),
					Retriable: false,
				})
			}
		}
	}()

	// Step 3: Execute the user-provided function with a new repository instance
	// The repository uses the transaction (tx) instead of the main DB connection
	// This ensures all operations within the function are part of the same transaction
	err = fc(ctx, New(tx))
	if err != nil {
		// Error will be handled by the defer block above (Step 2b)
		return err
	}

	// Step 4: Return nil on success
	// The defer block will commit the transaction (Step 2c)
	return nil
}
