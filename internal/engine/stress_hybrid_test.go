package engine_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/santosh-sankranthi/GIC/internal/engine"
	"github.com/santosh-sankranthi/GIC/internal/ir"
	"github.com/santosh-sankranthi/GIC/internal/loader"
)

type TestCaseResult struct {
	CaseID        string `json:"case_id"`
	Description   string `json:"description"`
	Passed        bool   `json:"passed"`
	Output        any    `json:"output,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	DurationMicro int64  `json:"duration_micro"`
	Behavior      string `json:"behavior"`
}

type WorkflowResult struct {
	WorkflowID  int              `json:"workflow_id"`
	Name        string           `json:"name"`
	Compiled    bool             `json:"compiled"`
	CompileErr  string           `json:"compile_err,omitempty"`
	TestResults []TestCaseResult `json:"test_results"`
}

type StressReport struct {
	Timestamp      string           `json:"timestamp"`
	TotalWorkflows int              `json:"total_workflows"`
	TotalTests     int              `json:"total_tests"`
	PassedTests    int              `json:"passed_tests"`
	FailedTests    int              `json:"failed_tests"`
	Workflows      []WorkflowResult `json:"workflows"`
}

type WorkflowDef struct {
	ID             int
	Name           string
	Decision       string
	Source         string
	InferNode      string
	SecondInfer    string
	NominalInputs  map[string]any
	MissingInputs  map[string]any
	NominalVal     any
	LowerBoundVal  any
	UpperBoundVal  any
	BelowDomainVal any
	AboveDomainVal any
	WrongTypeVal   any
}

func get15Workflows() []WorkflowDef {
	return []WorkflowDef{
		{
			ID:       1,
			Name:     "Credit Underwriting & Stability",
			Decision: "approve",
			InferNode: "text_stability_score",
			Source: `model "credit_underwriting" {}
input income : number in [1000..1000000]
input monthly_debt : number in [0..100000]
input credit_score : number in [300..850]
input app_narrative : string
infer text_stability_score : number in [0..1] { using: app_narrative }
decision dti : number = monthly_debt / income
decision approve : string {
  needs: dti, credit_score, text_stability_score
  hit: first
  # dti    | credit_score | text_stability_score => approve
    <= 0.3 | >= 700       | >= 0.7              => "APPROVED_PRIME"
    <= 0.4 | >= 650       | >= 0.5              => "APPROVED_STANDARD"
    -      | -            | -                   => "REJECTED"
}`,
			NominalInputs:  map[string]any{"income": 10000, "monthly_debt": 2000, "credit_score": 750, "app_narrative": "stable employment 5 years"},
			MissingInputs:  map[string]any{"monthly_debt": 2000, "credit_score": 750, "app_narrative": "stable"},
			NominalVal:     0.85,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1.0,
			BelowDomainVal: -0.5,
			AboveDomainVal: 1.5,
			WrongTypeVal:   "not_a_number",
		},
		{
			ID:       2,
			Name:     "Insurance Claim Fraud Detection",
			Decision: "payout_decision",
			InferNode: "fraud_probability",
			Source: `model "claim_fraud_detection" {}
input claim_amount : number in [1..1000000]
input police_report : boolean
input adjuster_notes : string
infer fraud_probability : number in [0..1] { using: adjuster_notes, claim_amount }
decision payout_decision : string {
  needs: fraud_probability, police_report, claim_amount
  hit: first
  # fraud_probability | police_report | claim_amount => payout_decision
    > 0.7             | -             | -            => "ESCALATE_SIU"
    <= 0.2            | true          | <= 5000      => "FAST_TRACK_PAY"
    <= 0.5            | true          | -            => "STANDARD_PAY"
    -                 | -             | -            => "MANUAL_REVIEW"
}`,
			NominalInputs:  map[string]any{"claim_amount": 3500, "police_report": true, "adjuster_notes": "damage matches description"},
			MissingInputs:  map[string]any{"claim_amount": 3500, "adjuster_notes": "clean"},
			NominalVal:     0.15,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1.0,
			BelowDomainVal: -0.2,
			AboveDomainVal: 1.8,
			WrongTypeVal:   map[string]any{"bad": "object"},
		},
		{
			ID:       3,
			Name:     "E-Commerce Dispute Mediation",
			Decision: "resolution",
			InferNode: "customer_sentiment",
			SecondInfer: "merchant_fault_score",
			Source: `model "dispute_resolution" {}
input order_age_days : number in [1..365]
input delivery_confirmed : boolean
input chat_transcript : string
infer customer_sentiment : string in ["angry", "neutral", "satisfied"] { using: chat_transcript }
infer merchant_fault_score : number in [0..1] { using: chat_transcript, delivery_confirmed }
decision resolution : string {
  needs: customer_sentiment, merchant_fault_score, order_age_days
  hit: first
  # customer_sentiment | merchant_fault_score | order_age_days => resolution
    "angry"            | >= 0.6               | -              => "FULL_REFUND_EXPEDITE"
    "angry"            | < 0.6                | <= 30          => "STORE_CREDIT"
    "neutral"          | >= 0.5               | -              => "PARTIAL_REFUND"
    -                  | -                    | -              => "DENY_REFUND"
}`,
			NominalInputs:  map[string]any{"order_age_days": 12, "delivery_confirmed": false, "chat_transcript": "package missing"},
			MissingInputs:  map[string]any{"delivery_confirmed": false, "chat_transcript": "package missing"},
			NominalVal:     "angry",
			LowerBoundVal:  "angry",
			UpperBoundVal:  "satisfied",
			BelowDomainVal: "furious_unknown",
			AboveDomainVal: "ecstatic_unknown",
			WrongTypeVal:   12345,
		},
		{
			ID:       4,
			Name:     "Medical Clinical Triage",
			Decision: "triage_category",
			InferNode: "acuity_index",
			Source: `model "clinical_triage" {}
input heart_rate : number in [30..250]
input spo2 : number in [50..100]
input triage_notes : string
infer acuity_index : number in [1..5] { using: triage_notes }
decision triage_category : string {
  needs: spo2, heart_rate, acuity_index
  hit: first
  # spo2  | heart_rate     | acuity_index => triage_category
    < 85  | -              | -            => "RESUSCITATION"
    < 92  | > 130          | <= 2         => "EMERGENT"
    -     | -              | <= 2         => "URGENT"
    -     | -              | -            => "NON_URGENT"
}`,
			NominalInputs:  map[string]any{"heart_rate": 88, "spo2": 97, "triage_notes": "mild cough"},
			MissingInputs:  map[string]any{"spo2": 97, "triage_notes": "mild cough"},
			NominalVal:     4.0,
			LowerBoundVal:  1.0,
			UpperBoundVal:  5.0,
			BelowDomainVal: 0.0,
			AboveDomainVal: 10.0,
			WrongTypeVal:   []string{"unexpected", "slice"},
		},
		{
			ID:       5,
			Name:     "KYC & Document Verification",
			Decision: "kyc_verdict",
			InferNode: "authenticity_confidence",
			Source: `model "kyc_verification" {}
input applicant_country : string in ["US", "CA", "GB", "DE", "OTHER"]
input document_type : string in ["PASSPORT", "DRIVERS_LICENSE", "NATIONAL_ID"]
infer authenticity_confidence : number in [0..1] { using: document_type }
decision kyc_verdict : string {
  needs: applicant_country, authenticity_confidence
  hit: first
  # applicant_country   | authenticity_confidence => kyc_verdict
    "US","CA","GB","DE" | >= 0.85                 => "PASS"
    "US","CA","GB","DE" | >= 0.60                 => "MANUAL_ID_CHECK"
    -                   | -                       => "FAIL"
}`,
			NominalInputs:  map[string]any{"applicant_country": "US", "document_type": "PASSPORT"},
			MissingInputs:  map[string]any{"document_type": "PASSPORT"},
			NominalVal:     0.94,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1.0,
			BelowDomainVal: -0.1,
			AboveDomainVal: 2.0,
			WrongTypeVal:   false,
		},
		{
			ID:       6,
			Name:     "Code Review Security Gate",
			Decision: "merge_verdict",
			InferNode: "security_vulnerability_risk",
			SecondInfer: "code_quality_grade",
			Source: `model "security_gate" {}
input lines_changed : number in [1..50000]
input files_touched : number in [1..500]
input pr_diff_text : string
infer security_vulnerability_risk : number in [0..10] { using: pr_diff_text }
infer code_quality_grade : string in ["A", "B", "C", "F"] { using: pr_diff_text, lines_changed }
decision merge_verdict : boolean {
  needs: security_vulnerability_risk, code_quality_grade, lines_changed
  hit: first
  # security_vulnerability_risk | code_quality_grade | lines_changed => merge_verdict
    >= 7                        | -                  | -             => false
    -                           | "F"                | -             => false
    <= 3                        | "A","B"            | -             => true
    -                           | -                  | -             => false
}`,
			NominalInputs:  map[string]any{"lines_changed": 120, "files_touched": 3, "pr_diff_text": "refactored utils"},
			MissingInputs:  map[string]any{"files_touched": 3, "pr_diff_text": "refactored utils"},
			NominalVal:     2.0,
			LowerBoundVal:  0.0,
			UpperBoundVal:  10.0,
			BelowDomainVal: -5.0,
			AboveDomainVal: 15.0,
			WrongTypeVal:   "two",
		},
		{
			ID:       7,
			Name:     "Dynamic Loan Pricing",
			Decision: "final_rate",
			InferNode: "market_adjustment",
			Source: `model "loan_pricing" {}
input base_rate : number in [0.01..0.30]
input loan_amount : number in [1000..1000000]
input borrower_risk_tier : string in ["TIER_1", "TIER_2", "TIER_3"]
infer market_adjustment : number in [-0.05..0.05] { using: loan_amount }
decision tier_margin : number {
  needs: borrower_risk_tier
  hit: first
  # borrower_risk_tier => tier_margin
    "TIER_1"           => 0.015
    "TIER_2"           => 0.035
    -                  => 0.070
}
decision final_rate : number = base_rate + tier_margin + market_adjustment`,
			NominalInputs:  map[string]any{"base_rate": 0.05, "loan_amount": 250000, "borrower_risk_tier": "TIER_1"},
			MissingInputs:  map[string]any{"loan_amount": 250000, "borrower_risk_tier": "TIER_1"},
			NominalVal:     0.01,
			LowerBoundVal:  -0.05,
			UpperBoundVal:  0.05,
			BelowDomainVal: -0.20,
			AboveDomainVal: 0.30,
			WrongTypeVal:   "high_rate",
		},
		{
			ID:       8,
			Name:     "Supply Chain Order Routing",
			Decision: "expedite_strategy",
			InferNode: "supplier_disruption_index",
			Source: `model "supply_chain_routing" {}
input inventory_level : number in [0..10000]
input lead_time_days : number in [1..90]
input vendor_bulletin : string
infer supplier_disruption_index : number in [0..1] { using: vendor_bulletin }
decision expedite_strategy : string {
  needs: supplier_disruption_index, inventory_level, lead_time_days
  hit: first
  # supplier_disruption_index | inventory_level | lead_time_days => expedite_strategy
    >= 0.7                    | < 100           | -              => "AIR_FREIGHT_URGENT"
    >= 0.5                    | < 500           | > 14           => "EXPEDITED_GROUND"
    -                         | >= 500          | -              => "STANDARD_SCHEDULE"
    -                         | -               | -              => "MONITOR"
}`,
			NominalInputs:  map[string]any{"inventory_level": 80, "lead_time_days": 20, "vendor_bulletin": "port strike announced"},
			MissingInputs:  map[string]any{"lead_time_days": 20, "vendor_bulletin": "port strike"},
			NominalVal:     0.82,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1.0,
			BelowDomainVal: -1.0,
			AboveDomainVal: 5.0,
			WrongTypeVal:   true,
		},
		{
			ID:       9,
			Name:     "Customer Churn Prevention",
			Decision: "retention_offer",
			InferNode: "churn_risk_score",
			Source: `model "churn_prevention" {}
input tenure_months : number in [1..120]
input monthly_spend : number in [1..10000]
input feedback_text : string
infer churn_risk_score : number in [0..1] { using: feedback_text, tenure_months }
decision retention_offer : string {
  needs: churn_risk_score, monthly_spend, tenure_months
  hit: first
  # churn_risk_score | monthly_spend | tenure_months => retention_offer
    >= 0.75          | >= 500        | -             => "DEDICATED_ACCOUNT_MANAGER_DISCOUNT"
    >= 0.60          | < 500         | >= 12         => "LOYALTY_CREDIT_VOUCHER"
    >= 0.40          | -             | -             => "PROACTIVE_SUPPORT_OUTREACH"
    -                | -             | -             => "NO_OFFER"
}`,
			NominalInputs:  map[string]any{"tenure_months": 24, "monthly_spend": 600, "feedback_text": "considering canceling service"},
			MissingInputs:  map[string]any{"monthly_spend": 600, "feedback_text": "considering canceling"},
			NominalVal:     0.78,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1.0,
			BelowDomainVal: -0.05,
			AboveDomainVal: 1.05,
			WrongTypeVal:   100,
		},
		{
			ID:       10,
			Name:     "Sequential Routing Pipeline",
			Decision: "routing_tier",
			InferNode: "intent_category",
			SecondInfer: "query_complexity",
			Source: `model "sequential_routing" {}
input customer_query : string
input account_age_days : number in [1..3650]
infer intent_category : string in ["REFUND", "TECHNICAL", "BILLING", "ACCOUNT"] { using: customer_query }
infer query_complexity : number in [1..10] { using: customer_query }
decision routing_tier : string {
  needs: intent_category, query_complexity, account_age_days
  hit: first
  # intent_category | query_complexity | account_age_days => routing_tier
    "REFUND"        | > 7              | -                => "TIER_3_FINANCE_SPECIALIST"
    "TECHNICAL"     | > 6              | -                => "TIER_2_ENGINEERING_SUPPORT"
    -               | <= 4             | -                => "TIER_1_GENERAL_SUPPORT"
    -               | -                | -                => "SENIOR_OMBUDSMAN"
}`,
			NominalInputs:  map[string]any{"customer_query": "need refund on charged subscription", "account_age_days": 180},
			MissingInputs:  map[string]any{"customer_query": "need refund"},
			NominalVal:     "REFUND",
			LowerBoundVal:  "REFUND",
			UpperBoundVal:  "ACCOUNT",
			BelowDomainVal: "UNKNOWN_INTENT_1",
			AboveDomainVal: "UNKNOWN_INTENT_2",
			WrongTypeVal:   nil,
		},
		{
			ID:       11,
			Name:     "AML Compliance Risk Screening",
			Decision: "sar_required",
			InferNode: "narrative_suspicion_score",
			Source: `model "aml_compliance" {}
input tx_amount : number in [1..10000000]
input is_pep : boolean
input swift_memo : string
infer narrative_suspicion_score : number in [0..100] { using: swift_memo }
decision sar_required : boolean {
  needs: narrative_suspicion_score, is_pep, tx_amount
  hit: first
  # narrative_suspicion_score | is_pep | tx_amount   => sar_required
    >= 80                     | -      | -           => true
    >= 50                     | true   | >= 10000    => true
    -                         | true   | >= 100000   => true
    -                         | -      | -           => false
}`,
			NominalInputs:  map[string]any{"tx_amount": 50000, "is_pep": true, "swift_memo": "consultancy payment via offshore entity"},
			MissingInputs:  map[string]any{"is_pep": true, "swift_memo": "consultancy payment"},
			NominalVal:     85.0,
			LowerBoundVal:  0.0,
			UpperBoundVal:  100.0,
			BelowDomainVal: -10.0,
			AboveDomainVal: 150.0,
			WrongTypeVal:   "suspicious",
		},
		{
			ID:       12,
			Name:     "Content Moderation Firewall",
			Decision: "action",
			InferNode: "toxicity_rating",
			SecondInfer: "misinformation_flag",
			Source: `model "content_moderation" {}
input user_karma : number in [-1000..100000]
input post_body : string
infer toxicity_rating : number in [0..1] { using: post_body }
infer misinformation_flag : boolean in [true, false] { using: post_body }
decision action : string {
  needs: toxicity_rating, misinformation_flag, user_karma
  hit: first
  # toxicity_rating | misinformation_flag | user_karma => action
    >= 0.85         | -                   | -          => "INSTANT_BAN"
    >= 0.60         | true                | -          => "SHADOWBAN_POST"
    -               | true                | < 100      => "ADD_COMMUNITY_WARNING"
    < 0.20          | false               | >= 50      => "APPROVE_POST"
    -               | -                   | -          => "FLAG_FOR_HUMAN_REVIEW"
}`,
			NominalInputs:  map[string]any{"user_karma": 500, "post_body": "informative community update"},
			MissingInputs:  map[string]any{"post_body": "informative update"},
			NominalVal:     0.05,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1.0,
			BelowDomainVal: -0.2,
			AboveDomainVal: 1.2,
			WrongTypeVal:   "clean",
		},
		{
			ID:       13,
			Name:     "Performance Bonus Calibration",
			Decision: "bonus_amount",
			InferNode: "leadership_index",
			Source: `model "performance_bonus" {}
input base_salary : number in [30000..500000]
input kpi_achievement_pct : number in [0..200]
input peer_360_feedback : string
infer leadership_index : number in [1..5] { using: peer_360_feedback }
decision multiplier : number {
  needs: kpi_achievement_pct, leadership_index
  hit: first
  # kpi_achievement_pct | leadership_index => multiplier
    >= 120              | >= 4.5           => 0.30
    >= 100              | >= 3.5           => 0.15
    >= 80               | >= 3.0           => 0.05
    -                   | -                => 0.00
}
decision bonus_amount : number = base_salary * multiplier`,
			NominalInputs:  map[string]any{"base_salary": 120000, "kpi_achievement_pct": 110, "peer_360_feedback": "exemplary team player and mentor"},
			MissingInputs:  map[string]any{"kpi_achievement_pct": 110, "peer_360_feedback": "great"},
			NominalVal:     4.0,
			LowerBoundVal:  1.0,
			UpperBoundVal:  5.0,
			BelowDomainVal: 0.5,
			AboveDomainVal: 6.0,
			WrongTypeVal:   "excellent",
		},
		{
			ID:       14,
			Name:     "Real Estate Valuation & LTV",
			Decision: "loan_approval",
			InferNode: "property_condition_discount",
			Source: `model "real_estate_valuation" {}
input list_price : number in [50000..5000000]
input requested_loan : number in [10000..5000000]
input inspection_report : string
infer property_condition_discount : number in [0..0.4] { using: inspection_report }
decision appraised_value : number = list_price * (1 - property_condition_discount)
decision loan_approval : boolean {
  needs: appraised_value, requested_loan
  hit: first
  # appraised_value   | requested_loan => loan_approval
    >= requested_loan | <= 2000000     => true
    -                 | -              => false
}`,
			NominalInputs:  map[string]any{"list_price": 500000, "requested_loan": 350000, "inspection_report": "minor wear, good roof"},
			MissingInputs:  map[string]any{"requested_loan": 350000, "inspection_report": "minor wear"},
			NominalVal:     0.10,
			LowerBoundVal:  0.0,
			UpperBoundVal:  0.40,
			BelowDomainVal: -0.10,
			AboveDomainVal: 0.80,
			WrongTypeVal:   "10%",
		},
		{
			ID:       15,
			Name:     "Smart Grid Energy Dispatch",
			Decision: "dispatch_strategy",
			InferNode: "solar_output_forecast_mw",
			Source: `model "smart_grid_dispatch" {}
input current_demand_mw : number in [10..5000]
input battery_soc_pct : number in [0..100]
input weather_forecast_text : string
infer solar_output_forecast_mw : number in [0..1000] { using: weather_forecast_text }
decision dispatch_strategy : string {
  needs: solar_output_forecast_mw, battery_soc_pct, current_demand_mw
  hit: first
  # solar_output_forecast_mw | battery_soc_pct | current_demand_mw => dispatch_strategy
    >= 600                   | >= 80           | -                 => "EXPORT_TO_NEIGHBORING_GRID"
    >= 400                   | -               | <= 1000           => "SOLAR_SELF_CONSUMPTION"
    < 200                    | < 20            | > 2000            => "ACTIVATE_PEAKER_PLANTS"
    -                        | >= 50           | -                 => "DISCHARGE_BATTERY_STORAGE"
    -                        | -               | -                 => "BALANCED_DISPATCH"
}`,
			NominalInputs:  map[string]any{"current_demand_mw": 800, "battery_soc_pct": 85, "weather_forecast_text": "sunny cloudless afternoon"},
			MissingInputs:  map[string]any{"battery_soc_pct": 85, "weather_forecast_text": "sunny"},
			NominalVal:     700.0,
			LowerBoundVal:  0.0,
			UpperBoundVal:  1000.0,
			BelowDomainVal: -100.0,
			AboveDomainVal: 2500.0,
			WrongTypeVal:   "700mw",
		},
	}
}

func makeRegistry(primaryNode string, primaryVal any, secondNode string, secondVal any, errNode string, err error) engine.ExternalRegistry {
	reg := engine.ExternalRegistry{}
	if primaryNode != "" {
		if errNode == primaryNode {
			reg[primaryNode] = func(inputs map[string]any) (any, error) {
				return nil, err
			}
		} else {
			reg[primaryNode] = func(inputs map[string]any) (any, error) {
				return primaryVal, nil
			}
		}
	}
	if secondNode != "" {
		if errNode == secondNode {
			reg[secondNode] = func(inputs map[string]any) (any, error) {
				return nil, err
			}
		} else {
			reg[secondNode] = func(inputs map[string]any) (any, error) {
				return secondVal, nil
			}
		}
	}
	return reg
}

func secondNominalDefault(secondNode string) any {
	switch secondNode {
	case "merchant_fault_score":
		return 0.75
	case "code_quality_grade":
		return "A"
	case "query_complexity":
		return 8.0
	case "misinformation_flag":
		return false
	default:
		return 0.5
	}
}

func isNull(v any) bool {
	if v == nil {
		return true
	}
	if iv, ok := v.(ir.Value); ok && iv.Tag == ir.TagNull {
		return true
	}
	return false
}

func Test15HybridWorkflows150Cases(t *testing.T) {
	workflows := get15Workflows()
	var report StressReport
	report.Timestamp = time.Now().UTC().Format(time.RFC3339)
	report.TotalWorkflows = len(workflows)

	for _, wf := range workflows {
		wfRes := WorkflowResult{
			WorkflowID: wf.ID,
			Name:       wf.Name,
		}

		// Compile
		cm, _, _, err := loader.CompileFile(fmt.Sprintf("wf_%d.rules", wf.ID), []byte(wf.Source))
		if err != nil {
			wfRes.Compiled = false
			wfRes.CompileErr = err.Error()
			t.Fatalf("Workflow %d (%s) failed to compile: %v", wf.ID, wf.Name, err)
		}
		wfRes.Compiled = true

		// Define 10 test cases
		testCases := []struct {
			id       string
			desc     string
			inputs   map[string]any
			reg      engine.ExternalRegistry
			validate func(out any, evalErr error) (bool, string)
		}{
			{
				id:     "T1_Nominal",
				desc:   "Nominal in-domain inputs & resolver return",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, wf.NominalVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					if evalErr != nil {
						return false, fmt.Sprintf("unexpected eval error: %v", evalErr)
					}
					if isNull(out) {
						return false, "unexpected null output on happy path"
					}
					return true, fmt.Sprintf("success output: %v", out)
				},
			},
			{
				id:     "T2_LowerBound",
				desc:   "Exact lower bound of infer domain",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, wf.LowerBoundVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					if evalErr != nil {
						return false, fmt.Sprintf("eval error on lower bound: %v", evalErr)
					}
					return true, fmt.Sprintf("lower bound evaluated cleanly: %v", out)
				},
			},
			{
				id:     "T3_UpperBound",
				desc:   "Exact upper bound of infer domain",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, wf.UpperBoundVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					if evalErr != nil {
						return false, fmt.Sprintf("eval error on upper bound: %v", evalErr)
					}
					return true, fmt.Sprintf("upper bound evaluated cleanly: %v", out)
				},
			},
			{
				id:     "T4_OutOfBoundsBelow",
				desc:   "Value strictly below infer domain minimum",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, wf.BelowDomainVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					// feelc engine passes through runtime values to VM, verifier checks domain at compile time.
					// Engine must not panic; either evaluates fallback rule or returns valid result.
					if evalErr != nil {
						return true, fmt.Sprintf("gracefully rejected/handled out-of-domain value: %v", evalErr)
					}
					return true, fmt.Sprintf("gracefully evaluated below-bound value to: %v", out)
				},
			},
			{
				id:     "T5_OutOfBoundsAbove",
				desc:   "Value strictly above infer domain maximum",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, wf.AboveDomainVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					if evalErr != nil {
						return true, fmt.Sprintf("gracefully rejected/handled out-of-domain value: %v", evalErr)
					}
					return true, fmt.Sprintf("gracefully evaluated above-bound value to: %v", out)
				},
			},
			{
				id:     "T6_ResolverError",
				desc:   "Resolver returns simulated network/third-party error",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, nil, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), wf.InferNode, errors.New("upstream timeout")),
				validate: func(out any, evalErr error) (bool, string) {
					// Engine design: resolver error maps to ir.Null() (null propagation), no panic!
					if evalErr != nil {
						return true, fmt.Sprintf("surfaced error as expected: %v", evalErr)
					}
					return true, fmt.Sprintf("propagated null successfully, table produced: %v", out)
				},
			},
			{
				id:     "T7_MissingDependency",
				desc:   "Missing required input in inputs map",
				inputs: wf.MissingInputs,
				reg:    makeRegistry(wf.InferNode, wf.NominalVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					// Missing input should coerce to null or produce error/null without panic
					return true, fmt.Sprintf("handled missing input without panic (out=%v, err=%v)", out, evalErr)
				},
			},
			{
				id:     "T8_NilRegistry",
				desc:   "engine.Eval called with nil registry on infer model",
				inputs: wf.NominalInputs,
				reg:    nil,
				validate: func(out any, evalErr error) (bool, string) {
					// Must return explicit error: "external registry required"
					if evalErr == nil {
						return false, "expected error for nil registry, got nil"
					}
					return true, fmt.Sprintf("safely rejected nil registry: %v", evalErr)
				},
			},
			{
				id:     "T9_MissingResolverKey",
				desc:   "Registry provided but missing resolver for this infer node",
				inputs: wf.NominalInputs,
				reg:    engine.ExternalRegistry{"completely_unrelated_key": func(map[string]any) (any, error) { return 0, nil }},
				validate: func(out any, evalErr error) (bool, string) {
					// Must return named error identifying the missing resolver
					if evalErr == nil {
						return false, "expected error for missing resolver key"
					}
					return true, fmt.Sprintf("safely caught missing resolver key: %v", evalErr)
				},
			},
			{
				id:     "T10_TypeMismatch",
				desc:   "Resolver returns incompatible Go type for declared infer type",
				inputs: wf.NominalInputs,
				reg:    makeRegistry(wf.InferNode, wf.WrongTypeVal, wf.SecondInfer, secondNominalDefault(wf.SecondInfer), "", nil),
				validate: func(out any, evalErr error) (bool, string) {
					// Coercion handles or flags invalid type cleanly without panic
					return true, fmt.Sprintf("handled type mismatch safely (out=%v, err=%v)", out, evalErr)
				},
			},
		}

		for _, tc := range testCases {
			start := time.Now()
			var out any
			var evalErr error
			var panicErr any

			func() {
				defer func() {
					if r := recover(); r != nil {
						panicErr = r
					}
				}()
				out, evalErr = engine.Eval(cm, wf.Decision, tc.inputs, tc.reg)
			}()

			dur := time.Since(start).Microseconds()
			report.TotalTests++

			if panicErr != nil {
				report.FailedTests++
				wfRes.TestResults = append(wfRes.TestResults, TestCaseResult{
					CaseID:        tc.id,
					Description:   tc.desc,
					Passed:        false,
					ErrorMessage:  fmt.Sprintf("PANIC: %v", panicErr),
					DurationMicro: dur,
					Behavior:      "UNEXPECTED_PANIC",
				})
				t.Errorf("CRITICAL: Workflow %d %s panicked on %s: %v", wf.ID, wf.Name, tc.id, panicErr)
				continue
			}

			pass, behavior := tc.validate(out, evalErr)
			if pass {
				report.PassedTests++
			} else {
				report.FailedTests++
				t.Errorf("Workflow %d %s failed on %s: %s", wf.ID, wf.Name, tc.id, behavior)
			}

			wfRes.TestResults = append(wfRes.TestResults, TestCaseResult{
				CaseID:        tc.id,
				Description:   tc.desc,
				Passed:        pass,
				Output:        out,
				ErrorMessage:  func() string { if evalErr != nil { return evalErr.Error() }; return "" }(),
				DurationMicro: dur,
				Behavior:      behavior,
			})
		}

		report.Workflows = append(report.Workflows, wfRes)
	}

	// Write full JSON report to disk
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		_ = os.WriteFile("stress_report_raw.json", reportJSON, 0644)
	}
}
