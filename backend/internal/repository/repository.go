package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound            = errors.New("record not found")
	ErrDuplicate           = errors.New("duplicate record")
	ErrForeignKeyViolation = errors.New("foreign key violation")
	ErrStateChanged        = errors.New("record state changed concurrently")
	ErrInvoiceNotEditable  = errors.New("invoice is not in draft status")
)

const (
	mysqlErrDuplicateEntry  = 1062
	mysqlErrRowIsReferenced = 1451
	mysqlErrNoReferencedRow = 1452
)

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case mysqlErrDuplicateEntry:
			return ErrDuplicate
		case mysqlErrRowIsReferenced, mysqlErrNoReferencedRow:
			return ErrForeignKeyViolation
		}
	}
	return err
}

func requireRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
