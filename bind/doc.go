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
// Every binder in this file (OneWay and the eight two-way binders below)
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
//
// Package bind also provides List[T], an observable slice (Add/Insert/
// RemoveAt/Set/Replace, with both coarse OnChanged and granular OnChange
// notifications), and Items, which keeps a StackPanel's children in sync
// with a List's contents by full rebuild on every change.
package bind
