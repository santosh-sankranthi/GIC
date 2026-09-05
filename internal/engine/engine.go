// Package engine is the facade that wires up the complete feelc pipeline:
// dsl.Parse -> compiler.Compile -> vm.Eval. It is the high-level entry point
// used by the CLI and by integration tests.
//
// For models that contain `infer` declarations (KindExternal decisions), Eval accepts
// an ExternalRegistry that maps node names to resolver functions. The resolver is called
// before vm.Eval so the VM always sees a fully populated inputs map — KindExternal nodes
// are resolved into plain ir.Values before the bytecode engine runs.
package engine

import (
	"fmt"

	"github.com/santosh-sankranthi/GIC/internal/compiler"
	"github.com/santosh-sankranthi/GIC/internal/dsl"
	"github.com/santosh-sankranthi/GIC/internal/ir"
	"github.com/santosh-sankranthi/GIC/internal/vm"
)

// ExternalRegistry maps infer node names to their resolver functions.
//
// The resolver receives the raw (pre-coercion) inputs that the infer node declared in
// its `using:` list, and must return a value compatible with the declared type. If the
// resolver returns an error, the node's value is mapped to ir.Null() — existing null
// propagation semantics handle it gracefully throughout the rule graph.
//
// The map key is the infer node name (exactly as declared in the `.rules` file).
type ExternalRegistry map[string]func(inputs map[string]any) (any, error)

// Run compiles a .rules source and evaluates a decision against raw inputs.
// Use for deterministic-only models. For models with `infer` nodes, use Eval directly.
func Run(src, decision string, rawInputs map[string]any) (any, error) {
	m, err := dsl.Parse(src)
	if err != nil {
		return nil, err
	}
	cm, err := compiler.Compile(m)
	if err != nil {
		return nil, err
	}
	return Eval(cm, decision, rawInputs, nil)
}

// Eval evaluates a decision of an ALREADY COMPILED model against raw inputs (JSON-ish).
// It is the service entry point (no per-request recompilation).
//
// registry provides resolvers for KindExternal (infer) nodes. Pass nil if the model has
// no infer declarations. If the model has infer nodes and registry is nil, Eval returns
// an error naming the first unresolvable node.
func Eval(cm *ir.CompiledModel, decision string, rawInputs map[string]any, registry ExternalRegistry) (any, error) {
	inputs := make(map[string]ir.Value, len(rawInputs))
	for k, v := range rawInputs {
		val, err := ir.FromAny(v)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", k, err)
		}
		inputs[k] = val
	}
	if err := ir.CoerceInputs(cm, inputs); err != nil {
		return nil, err
	}

	// Pre-eval enrichment: resolve all KindExternal decisions in topological order
	// (the compiler guarantees infer nodes precede the decisions that depend on them
	// in cm.Decisions). Each resolved value is injected into the inputs map so the VM
	// treats it as a regular input — the VM itself is completely unchanged.
	for i := range cm.Decisions {
		d := &cm.Decisions[i]
		if d.Kind != ir.KindExternal {
			continue
		}
		// Build the subset of raw inputs the resolver needs (the Using list = d.Deps).
		resolverInputs := make(map[string]any, len(d.Deps))
		for _, dep := range d.Deps {
			if v, ok := rawInputs[dep]; ok {
				resolverInputs[dep] = v
			}
		}
		// Require a resolver — nil registry on a model with infer nodes is misconfiguration.
		if registry == nil {
			return nil, fmt.Errorf("infer node %q: no ExternalRegistry provided (model has `infer` declarations)", d.Name)
		}
		resolver, ok := registry[d.Name]
		if !ok {
			return nil, fmt.Errorf("infer node %q: no resolver registered in ExternalRegistry", d.Name)
		}
		raw, resolveErr := resolver(resolverInputs)
		if resolveErr != nil {
			// Resolver failed → inject null; downstream null-propagation handles it.
			inputs[d.Name] = ir.Null()
		} else {
			val, convErr := ir.FromAny(raw)
			if convErr != nil {
				// Conversion failed → also null.
				inputs[d.Name] = ir.Null()
			} else {
				inputs[d.Name] = val
			}
		}
	}

	out, err := vm.Eval(cm, decision, inputs)
	if err != nil {
		return nil, err
	}
	return out.ToAny(), nil
}

