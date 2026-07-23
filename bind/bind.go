// Package bind connects core.Property[T] values to controls: one-way
// binding pushes a property's value into a control on every change, and
// two-way binding additionally OWNS a control's OnChanged slot so user edits
// flow back into the property.
//
// Naming: among the checked-family two-way binders, Checked names
// CheckBox's binder (the package's canonical bool control); the others are
// prefixed by their control (SwitchChecked, ToggleChecked) since Checked was
// already taken. This is a naming-only distinction — every two-way binder
// below, whatever it's called, follows the identical shared mechanics.
//
// Every binder in this file (OneWay and the seven two-way binders below)
// shares the same normative mechanics:
//
//   - On bind: the control is set to the property's CURRENT value via its
//     silent setter (SetText/SetChecked/SetValue/SetSelectedIndex — never
//     firing OnChanged, per the controls package's uniform setter
//     convention), then (two-way only) the binder installs its own
//     OnChanged callback that pushes user edits into the property via
//     p.Set, and subscribes to the property so any OTHER change (a "model
//     push") is applied back to the control via the same silent setter.
//   - Echo safety: a model push's silent setter call never re-fires the
//     control's OnChanged (SetText/SetChecked/SetValue/SetSelectedIndex
//     are all silent by construction), so there is no feedback loop back
//     into p.Set. Symmetrically, a user edit's p.Set(v) — when v already
//     equals p's current value — is a no-op per Property.Set's own
//     equality guard, so the binder's own p.OnChange subscriber never
//     re-applies a value the control already shows.
//   - cancel() detaches both directions: it cancels the property
//     subscription and clears the control's OnChanged slot (nil). Both
//     underlying operations are already idempotent (Property's cancel
//     deletes from a map by id; setting OnChanged(nil) again is a no-op),
//     so the composed cancel is idempotent too — calling it more than once
//     is a harmless no-op.
//
// Rebind caveat: binding the same control twice without canceling the first
// is undefined: the second bind takes over OnChanged ownership, and the
// first bind's cancel will clear the second's hook.
package bind

import (
	"github.com/0xdreadnaught/fluo/controls"
	"github.com/0xdreadnaught/fluo/core"
)

// OneWay applies p's current value via apply immediately, then re-applies
// apply(v) every time p changes thereafter. Returns a cancel func (Property.
// OnChange's own cancel, which is idempotent) that detaches the
// subscription; further changes to p after cancel no longer invoke apply.
func OneWay[T comparable](p *core.Property[T], apply func(T)) (cancel func()) {
	apply(p.Get())
	return p.OnChange(func(_, v T) { apply(v) })
}

// Text two-way binds p to tb (see the package doc comment for the shared
// mechanics). While bound, Text OWNS tb's OnChanged slot, replacing any
// previously set callback.
func Text(p *core.Property[string], tb *controls.TextBox) (cancel func()) {
	tb.SetText(p.Get())
	tb.OnChanged(func(v string) { p.Set(v) })
	sub := p.OnChange(func(_, v string) { tb.SetText(v) })
	return func() {
		sub()
		tb.OnChanged(nil)
	}
}

// Checked two-way binds p to cb (see the package doc comment for the shared
// mechanics). While bound, Checked OWNS cb's OnChanged slot, replacing any
// previously set callback.
func Checked(p *core.Property[bool], cb *controls.CheckBox) (cancel func()) {
	cb.SetChecked(p.Get())
	cb.OnChanged(func(v bool) { p.Set(v) })
	sub := p.OnChange(func(_, v bool) { cb.SetChecked(v) })
	return func() {
		sub()
		cb.OnChanged(nil)
	}
}

// SwitchChecked two-way binds p to sw (see the package doc comment for the
// shared mechanics). While bound, SwitchChecked OWNS sw's OnChanged slot,
// replacing any previously set callback.
func SwitchChecked(p *core.Property[bool], sw *controls.ToggleSwitch) (cancel func()) {
	sw.SetChecked(p.Get())
	sw.OnChanged(func(v bool) { p.Set(v) })
	sub := p.OnChange(func(_, v bool) { sw.SetChecked(v) })
	return func() {
		sub()
		sw.OnChanged(nil)
	}
}

// ToggleChecked two-way binds p to tb (see the package doc comment for the
// shared mechanics). While bound, ToggleChecked OWNS tb's OnChanged slot,
// replacing any previously set callback.
func ToggleChecked(p *core.Property[bool], tb *controls.ToggleButton) (cancel func()) {
	tb.SetChecked(p.Get())
	tb.OnChanged(func(v bool) { p.Set(v) })
	sub := p.OnChange(func(_, v bool) { tb.SetChecked(v) })
	return func() {
		sub()
		tb.OnChanged(nil)
	}
}

// Value two-way binds p to s (see the package doc comment for the shared
// mechanics). While bound, Value OWNS s's OnChanged slot, replacing any
// previously set callback.
//
// Clamp-divergence caveat: if p's value falls outside the control's valid
// range, the control silently displays a clamped value while p retains the
// unclamped one.
func Value(p *core.Property[float32], s *controls.Slider) (cancel func()) {
	s.SetValue(p.Get())
	s.OnChanged(func(v float32) { p.Set(v) })
	sub := p.OnChange(func(_, v float32) { s.SetValue(v) })
	return func() {
		sub()
		s.OnChanged(nil)
	}
}

// SelectedIndex two-way binds p to cb (see the package doc comment for the
// shared mechanics). While bound, SelectedIndex OWNS cb's OnChanged slot,
// replacing any previously set callback.
//
// Clamp-divergence caveat: if p's value falls outside the control's valid
// range, the control silently displays a clamped value while p retains the
// unclamped one.
func SelectedIndex(p *core.Property[int], cb *controls.ComboBox) (cancel func()) {
	cb.SetSelectedIndex(p.Get())
	cb.OnChanged(func(v int) { p.Set(v) })
	sub := p.OnChange(func(_, v int) { cb.SetSelectedIndex(v) })
	return func() {
		sub()
		cb.OnChanged(nil)
	}
}

// Selected two-way binds p to g (see the package doc comment for the shared
// mechanics). While bound, Selected OWNS g's OnChanged slot — the group's
// own OnChanged(index), not any individual member RadioButton's — replacing
// any previously set callback.
//
// Clamp-divergence caveat: if p's value falls outside the group's valid
// member range, the group silently displays a clamped selection (or none,
// for -1) while p retains the unclamped one.
func Selected(p *core.Property[int], g *controls.RadioGroup) (cancel func()) {
	g.SetSelectedIndex(p.Get())
	g.OnChanged(func(v int) { p.Set(v) })
	sub := p.OnChange(func(_, v int) { g.SetSelectedIndex(v) })
	return func() {
		sub()
		g.OnChanged(nil)
	}
}

// ListSelected two-way binds p to lv (see the package doc comment for the
// shared mechanics). While bound, ListSelected OWNS lv's OnChanged slot,
// replacing any previously set callback.
//
// Clamp-divergence caveat: if p's value falls outside the list's valid
// range, the control silently displays a clamped selection (or none, for
// -1) while p retains the unclamped one.
//
// Auto-scroll note: lv.SetSelectedIndex — the silent setter this binder's
// model-push direction calls, both on the initial apply and on every
// subsequent p.OnChange — ALSO scrolls the newly-selected row into view
// (see ListView.scrollIntoView's doc comment). So unlike every other
// two-way binder above, a ListSelected model push doesn't just silently
// update what the control reports selected — it can also move the
// viewport, exactly as a user-driven Home/End/Up/Down move would.
func ListSelected(p *core.Property[int], lv *controls.ListView) (cancel func()) {
	lv.SetSelectedIndex(p.Get())
	lv.OnChanged(func(v int) { p.Set(v) })
	sub := p.OnChange(func(_, v int) { lv.SetSelectedIndex(v) })
	return func() {
		sub()
		lv.OnChanged(nil)
	}
}
