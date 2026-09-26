package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"delmos/internal/auth"
	"delmos/internal/economics"
)

// Команди запису проєктної економіки. Без них таблиці, ставки й кошториси
// існували лише в Go, а шлюз ліміту фінансування був недосяжний у працюючій
// системі: ввести витрату чи затвердити кошторис не було чим (SHR-13).

type workRecordRequest struct {
	UserID        *uuid.UUID `json:"user_id"`
	WorkProductID *uuid.UUID `json:"work_product_id"`
	PhaseKey      string     `json:"phase_key"`
	RoleKey       string     `json:"role_key"`
	WorkDate      string     `json:"work_date"`
	DurationHours string     `json:"duration_hours"`
	WorkCategory  string     `json:"work_category"`
	Comment       string     `json:"comment"`
}

// handleLogWorkRecord фіксує відпрацьовані години. user_id необов'язковий:
// без нього списання йде на самого актора — типовий випадок табеля.
func handleLogWorkRecord(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, projectID, ok := projectScopePermission(w, r, authSvc, "timesheet.log")
		if !ok {
			return
		}
		var req workRecordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		workDate, err := time.Parse(time.DateOnly, req.WorkDate)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_work_date", "work_date має бути датою YYYY-MM-DD")
			return
		}
		subject := actor
		if req.UserID != nil {
			subject = *req.UserID
		}

		record, err := store.LogWorkRecord(r.Context(), economics.WorkRecord{
			ProjectID: projectID, UserID: subject, WorkProductID: req.WorkProductID,
			PhaseKey: req.PhaseKey, RoleKey: req.RoleKey, WorkDate: workDate,
			DurationHours: req.DurationHours, WorkCategory: req.WorkCategory, Comment: req.Comment,
		})
		switch {
		case err == nil:
			writeJSON(w, http.StatusCreated, record)
		case errors.Is(err, economics.ErrPhaseNotFound):
			writeJSONError(w, http.StatusNotFound, "phase_not_found", "фазу не знайдено в проєкті")
		case errors.Is(err, economics.ErrPhaseNotOpen):
			writeJSONError(w, http.StatusUnprocessableEntity, "phase_not_open", err.Error())
		default:
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_work_record", err.Error())
		}
	}
}

type laborRateRequest struct {
	RoleKey    string `json:"role_key"`
	HourlyRate string `json:"hourly_rate"`
	Currency   string `json:"currency"`
	ValidFrom  string `json:"valid_from"`
	ValidTo    string `json:"valid_to"`
}

func handleSetLaborRate(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, projectID, ok := projectScopePermission(w, r, authSvc, "economics.manage")
		if !ok {
			return
		}
		var req laborRateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		validFrom, err := time.Parse(time.DateOnly, req.ValidFrom)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_valid_from", "valid_from має бути датою YYYY-MM-DD")
			return
		}
		var validTo *time.Time
		if req.ValidTo != "" {
			parsed, err := time.Parse(time.DateOnly, req.ValidTo)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid_valid_to", "valid_to має бути датою YYYY-MM-DD")
				return
			}
			validTo = &parsed
		}

		id, err := store.SetLaborRate(r.Context(), &projectID, req.RoleKey, req.HourlyRate, req.Currency, validFrom, validTo)
		switch {
		case err == nil:
			writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
		case errors.Is(err, economics.ErrRatePeriodOverlap):
			writeJSONError(w, http.StatusConflict, "rate_period_overlap", err.Error())
		default:
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_labor_rate", err.Error())
		}
	}
}

type costBaselineRequest struct {
	Name     string                 `json:"name"`
	Currency string                 `json:"currency"`
	Lines    []economics.BudgetLine `json:"lines"`
}

func handleCreateCostBaseline(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, projectID, ok := projectScopePermission(w, r, authSvc, "economics.manage")
		if !ok {
			return
		}
		var req costBaselineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		id, err := store.CreateCostBaseline(r.Context(), actor, projectID, req.Name, req.Currency, req.Lines)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_cost_baseline", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
	}
}

// handleApproveCostBaseline затверджує версію кошторису. Право
// economics.approve відокремлене від economics.manage, а складач кошторису
// не може бути його затверджувачем (ADR-006, розподіл обов'язків).
func handleApproveCostBaseline(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, _, ok := projectScopePermission(w, r, authSvc, "economics.approve")
		if !ok {
			return
		}
		baselineID, err := uuid.Parse(r.PathValue("baseline_id"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_baseline_id", "некоректний ідентифікатор кошторису")
			return
		}

		err = store.ApproveCostBaseline(r.Context(), baselineID, actor)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
		case errors.Is(err, economics.ErrBaselineNotFound):
			writeJSONError(w, http.StatusNotFound, "baseline_not_found", "версію кошторису не знайдено")
		case errors.Is(err, economics.ErrSelfApproval):
			writeJSONError(w, http.StatusUnprocessableEntity, "self_approval", err.Error())
		case errors.Is(err, economics.ErrBaselineApproved):
			writeJSONError(w, http.StatusConflict, "already_approved", err.Error())
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "не вдалося затвердити кошторис")
		}
	}
}

type expenseRequest struct {
	PhaseKey         string `json:"phase_key"`
	CostCategory     string `json:"cost_category"`
	ExpenseType      string `json:"expense_type"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	InvoiceReference string `json:"invoice_reference"`
	ExpenseDate      string `json:"expense_date"`
}

func handleRecordExpense(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, projectID, ok := projectScopePermission(w, r, authSvc, "economics.manage")
		if !ok {
			return
		}
		var req expenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		expenseDate, err := time.Parse(time.DateOnly, req.ExpenseDate)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_expense_date", "expense_date має бути датою YYYY-MM-DD")
			return
		}
		id, err := store.RecordExpense(r.Context(), projectID, actor, req.PhaseKey,
			req.CostCategory, req.ExpenseType, req.Amount, req.Currency, req.InvoiceReference, expenseDate)
		switch {
		case err == nil:
			writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
		case errors.Is(err, economics.ErrPhaseNotFound):
			writeJSONError(w, http.StatusNotFound, "phase_not_found", "фазу не знайдено в проєкті")
		case errors.Is(err, economics.ErrPhaseNotOpen):
			writeJSONError(w, http.StatusUnprocessableEntity, "phase_not_open", err.Error())
		case errors.Is(err, economics.ErrCurrencyMismatch):
			writeJSONError(w, http.StatusUnprocessableEntity, "currency_mismatch", err.Error())
		default:
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_expense", err.Error())
		}
	}
}

type phaseDeliverableRequest struct {
	PhaseKey      string    `json:"phase_key"`
	WorkProductID uuid.UUID `json:"work_product_id"`
	Weight        string    `json:"weight"`
}

// handleLinkPhaseDeliverable приписує результат до фази з ваговим
// коефіцієнтом. Без цього звʼязку здобута цінність фази не обчислюється.
func handleLinkPhaseDeliverable(authSvc *auth.Service, store *economics.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, projectID, ok := projectScopePermission(w, r, authSvc, "economics.manage")
		if !ok {
			return
		}
		var req phaseDeliverableRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body", "некоректне тіло запиту")
			return
		}
		weight := req.Weight
		if weight == "" {
			weight = "1.000"
		}
		if err := store.LinkPhaseDeliverable(r.Context(), projectID, req.WorkProductID, actorID, req.PhaseKey, weight); err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "invalid_deliverable_link", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "linked"})
	}
}
