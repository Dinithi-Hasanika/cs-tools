// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CustomDashboardShareRepository reads and writes who a saved custom dashboard
// is shared with (migration 0250). The owner side takes the owner and only
// ever touches that owner's dashboards; the recipient side takes the
// recipient and only ever sees dashboards shared with them. ids must be UUIDs;
// the service checks that first, and that the recipient may receive a share.
type CustomDashboardShareRepository interface {
	// ListShares is who the owner's dashboard is shared with, ordered by name.
	// Empty both when there are none and when the dashboard is not the owner's.
	ListShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardShare, error)
	// AddShare shares the owner's dashboard with a user; sharing it again is not
	// an error. false when the owner has no dashboard with that id; a
	// NotFoundError when there is no such user.
	AddShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error)
	// RemoveShare stops sharing the owner's dashboard with a user; false when it
	// was not shared with them (or is not the owner's).
	RemoveShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error)
	// ListSharedWithUser is every dashboard shared with the user, without
	// definitions, ordered by name.
	ListSharedWithUser(ctx context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error)
	// GetSharedWithUser is one dashboard shared with the user; nil when it is not.
	GetSharedWithUser(ctx context.Context, userID, dashboardID string) (*domain.SharedCustomDashboard, error)
}

type customDashboardShareRepo struct {
	db *pgxpool.Pool
}

// NewCustomDashboardShareRepository constructs a Postgres-backed CustomDashboardShareRepository.
func NewCustomDashboardShareRepository(db *pgxpool.Pool) CustomDashboardShareRepository {
	return &customDashboardShareRepo{db: db}
}

// dashboardShareUserName is a user's display name, as elsewhere in this package.
const dashboardShareUserName = `COALESCE(%[1]s.name, NULLIF(TRIM(CONCAT_WS(' ', %[1]s.first_name, %[1]s.last_name)), ''), '')`

func dashboardShareUserRef(id, email, name string) domain.UserReference {
	if ref := domain.NewUserReference(id, email, name); ref != nil {
		return *ref
	}
	return domain.UserReference{}
}

func (r *customDashboardShareRepo) ListShares(ctx context.Context, ownerID, dashboardID string) ([]domain.CustomDashboardShare, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id::text, COALESCE(u.email, ''), `+fmt.Sprintf(dashboardShareUserName, "u")+`, s.created_on
		 FROM custom_dashboard_share s
		 JOIN custom_dashboard d ON d.id = s.dashboard_id
		 JOIN "user" u ON u.id = s.shared_with_user_id
		 WHERE d.id = $2::uuid AND d.owner_user_id = $1::uuid
		 ORDER BY lower(`+fmt.Sprintf(dashboardShareUserName, "u")+`), u.id`,
		ownerID, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("list custom dashboard shares: %w", err)
	}
	defer rows.Close()
	out := []domain.CustomDashboardShare{}
	for rows.Next() {
		var id, email, name string
		var sh domain.CustomDashboardShare
		if err := rows.Scan(&id, &email, &name, &sh.SharedOn); err != nil {
			return nil, fmt.Errorf("list custom dashboard shares: scan: %w", err)
		}
		sh.User = dashboardShareUserRef(id, email, name)
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list custom dashboard shares: %w", err)
	}
	return out, nil
}

func (r *customDashboardShareRepo) AddShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error) {
	// One statement: the insert only happens for the owner's own dashboard,
	// and the result says whether that dashboard exists, shared before or not.
	var owned bool
	err := r.db.QueryRow(ctx,
		`WITH d AS (
		   SELECT id FROM custom_dashboard WHERE id = $2::uuid AND owner_user_id = $1::uuid
		 ), ins AS (
		   INSERT INTO custom_dashboard_share (dashboard_id, shared_with_user_id)
		   SELECT id, $3::uuid FROM d
		   ON CONFLICT (dashboard_id, shared_with_user_id) DO NOTHING
		   RETURNING 1
		 )
		 SELECT EXISTS (SELECT 1 FROM d)`,
		ownerID, dashboardID, userID).Scan(&owned)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "custom_dashboard_share_shared_with_user_id_fkey" {
			return false, &apierror.NotFoundError{Msg: "user not found"}
		}
		return false, fmt.Errorf("add custom dashboard share: %w", err)
	}
	return owned, nil
}

func (r *customDashboardShareRepo) RemoveShare(ctx context.Context, ownerID, dashboardID, userID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM custom_dashboard_share s
		 USING custom_dashboard d
		 WHERE s.dashboard_id = d.id
		   AND d.id = $2::uuid AND d.owner_user_id = $1::uuid
		   AND s.shared_with_user_id = $3::uuid`,
		ownerID, dashboardID, userID)
	if err != nil {
		return false, fmt.Errorf("remove custom dashboard share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// sharedWithUserFrom is the join both recipient reads use: the dashboards
// shared with $1, each with its owner.
const sharedWithUserFrom = `
		 FROM custom_dashboard_share s
		 JOIN custom_dashboard d ON d.id = s.dashboard_id
		 JOIN "user" o ON o.id = d.owner_user_id
		 WHERE s.shared_with_user_id = $1::uuid`

func (r *customDashboardShareRepo) ListSharedWithUser(ctx context.Context, userID string) ([]domain.SharedCustomDashboardSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT d.id::text, d.name, o.id::text, COALESCE(o.email, ''), `+fmt.Sprintf(dashboardShareUserName, "o")+`, s.created_on, d.updated_on`+
			sharedWithUserFrom+`
		 ORDER BY lower(d.name), d.id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list dashboards shared with user: %w", err)
	}
	defer rows.Close()
	out := []domain.SharedCustomDashboardSummary{}
	for rows.Next() {
		var ownerID, ownerEmail, ownerName string
		var d domain.SharedCustomDashboardSummary
		if err := rows.Scan(&d.ID, &d.Name, &ownerID, &ownerEmail, &ownerName, &d.SharedOn, &d.UpdatedOn); err != nil {
			return nil, fmt.Errorf("list dashboards shared with user: scan: %w", err)
		}
		d.Owner = dashboardShareUserRef(ownerID, ownerEmail, ownerName)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list dashboards shared with user: %w", err)
	}
	return out, nil
}

func (r *customDashboardShareRepo) GetSharedWithUser(ctx context.Context, userID, dashboardID string) (*domain.SharedCustomDashboard, error) {
	var ownerID, ownerEmail, ownerName string
	var d domain.SharedCustomDashboard
	err := r.db.QueryRow(ctx,
		`SELECT d.id::text, d.name, d.definition, o.id::text, COALESCE(o.email, ''), `+fmt.Sprintf(dashboardShareUserName, "o")+`, s.created_on, d.updated_on`+
			sharedWithUserFrom+`
		   AND d.id = $2::uuid`,
		userID, dashboardID).Scan(&d.ID, &d.Name, &d.Definition, &ownerID, &ownerEmail, &ownerName, &d.SharedOn, &d.UpdatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get dashboard shared with user: %w", err)
	}
	d.Owner = dashboardShareUserRef(ownerID, ownerEmail, ownerName)
	return &d, nil
}
