package engine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/santosh-sankranthi/GIC/internal/compiler"
	"github.com/santosh-sankranthi/GIC/internal/dsl"
	"github.com/santosh-sankranthi/GIC/internal/engine"
	"github.com/santosh-sankranthi/GIC/internal/ir"
)

// mustParsedCompile parses + compiles a .rules source, fataling the test on error.
func mustParsedCompile(t *testing.T, src string) *ir.CompiledModel {
	t.Helper()
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cm, err := compiler.Compile(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return cm
}

// inferSrc: a model with one infer node (financial_stability) and one downstream
// literal-expression decision that consumes it.
const inferSrc = `model "test" {}
input income        : number >= 0
input credit_score  : number in [300..850]
input monthly_debt  : number >= 0

infer financial_stability : number in [0..1] {
  using: income, credit_score, monthly_debt
}

decision approve : boolean = income > 50000 and financial_stability >= 0.7`

func TestInferHappyPath(t *testing.T) {
	cm := mustParsedCompile(t, inferSrc)
	reg := engine.ExternalRegistry{
		"financial_stability": func(inputs map[string]any) (any, error) {
			return 0.73, nil
		},
	}
	got, err := engine.Eval(cm, "approve", map[string]any{
		"income": 60000, "credit_score": 720, "monthly_debt": 1200,
	}, reg)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != true {
		t.Errorf("approve = %v, want true", got)
	}
}

func TestInferLowScore(t *testing.T) {
	cm := mustParsedCompile(t, inferSrc)
	reg := engine.ExternalRegistry{
		"financial_stability": func(_ map[string]any) (any, error) { return 0.5, nil },
	}
	got, err := engine.Eval(cm, "approve", map[string]any{
		"income": 60000, "credit_score": 720, "monthly_debt": 1200,
	}, reg)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != false {
		t.Errorf("approve = %v, want false (stability 0.5 < 0.7)", got)
	}
}

func TestInferResolverErrorMapsToNull(t *testing.T) {
	cm := mustParsedCompile(t, inferSrc)
	reg := engine.ExternalRegistry{
		"financial_stability": func(_ map[string]any) (any, error) {
			return nil, errors.New("llm timeout")
		},
	}
	got, err := engine.Eval(cm, "approve", map[string]any{
		"income": 60000, "credit_score": 720, "monthly_debt": 1200,
	}, reg)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	// null propagation: stability=null -> comparison yields null -> and yields null -> false
	if got != false {
		t.Errorf("approve = %v, want false (resolver error maps to null)", got)
	}
}

func TestInferNilRegistryErrors(t *testing.T) {
	cm := mustParsedCompile(t, inferSrc)
	_, err := engine.Eval(cm, "approve", map[string]any{
		"income": 60000, "credit_score": 720, "monthly_debt": 1200,
	}, nil)
	if err == nil {
		t.Fatal("expected error for nil registry, got nil")
	}
	if !strings.Contains(err.Error(), "financial_stability") {
		t.Errorf("error = %q, want it to name the infer node", err.Error())
	}
}

func TestInferMissingResolverErrors(t *testing.T) {
	cm := mustParsedCompile(t, inferSrc)
	reg := engine.ExternalRegistry{
		"some_other_node": func(_ map[string]any) (any, error) { return 0.5, nil },
	}
	_, err := engine.Eval(cm, "approve", map[string]any{
		"income": 60000, "credit_score": 720, "monthly_debt": 1200,
	}, reg)
	if err == nil {
		t.Fatal("expected error for missing resolver, got nil")
	}
	if !strings.Contains(err.Error(), "financial_stability") {
		t.Errorf("error = %q, want it to name the missing infer node", err.Error())
	}
}

func TestInferCompilerRejectsUnknownUsing(t *testing.T) {
	src := "model \"m\" {}\ninput income : number\ninfer stability : number in [0..1] {\n  using: income, nonexistent_input\n}\ndecision d : boolean = stability >= 0.5"
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = compiler.Compile(m)
	if err == nil {
		t.Fatal("expected compile error for unknown using dep, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_input") {
		t.Errorf("error = %q, want it to name the unknown dep", err.Error())
	}
}

func TestInferCompilerRejectsDuplicateName(t *testing.T) {
	src := "model \"m\" {}\ninput income : number\ninfer income : number in [0..1] {\n  using: income\n}\ndecision d : boolean = income >= 0.5"
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = compiler.Compile(m)
	if err == nil {
		t.Fatal("expected compile error for duplicate name, got nil")
	}
}

func TestInferInlineSelfClosed(t *testing.T) {
	src := "model \"m\" {}\ninput income : number >= 0\ninfer score : number in [0..1] { using: income }\ndecision ok : boolean = score >= 0.5"
	cm := mustParsedCompile(t, src)
	reg := engine.ExternalRegistry{
		"score": func(_ map[string]any) (any, error) { return 0.8, nil },
	}
	got, err := engine.Eval(cm, "ok", map[string]any{"income": 50000}, reg)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != true {
		t.Errorf("ok = %v, want true", got)
	}
}
