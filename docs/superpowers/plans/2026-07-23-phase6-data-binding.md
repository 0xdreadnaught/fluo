# fluo Phase 6: Data Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The `bind` package: one-way Property→UI application, echo-safe two-way binding for every input control (TextBox/CheckBox/ToggleSwitch/ToggleButton/Slider/ComboBox), and ItemsSource collection binding via an observable list — demonstrated live in the gallery.

**Architecture:** Built entirely on Phase 5's uniform contract: programmatic setters are SILENT, `OnChanged` fires only for user-driven changes. Two-way binding is therefore echo-safe by construction: model→control pushes use silent setters (no OnChanged); control→model pushes go through `Property.Set` whose equality guard stops loops. The `bind` package OWNS the control's single `OnChanged` slot while bound (documented loudly — users observe through the Property). Collection binding v0 = full child rebuild on any change (virtualization is Phase 7).

**Tech Stack:** Pure Go, pinned deps. `bind` imports `core`, `controls`, stdlib. Everything headless-testable; no new goldens.

## Global Constraints

- All prior constraints bind (go.mod PINNED; WSL-only Go; keyed literals; vet+gofmt; doc comments; per-task commits + trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`; goldens untouched).
- `bind` must not need GL; no reflection, no interface{} — concrete typed functions per control (generics only where clean: OneWay, List).
- Every binder returns `cancel func()`: idempotent, detaches the Property subscription AND clears the control's OnChanged slot (sets nil callback).
- Binders apply the Property's CURRENT value to the control immediately on bind (model is the source of truth at bind time).

## File Structure

```
bind/
├── bind.go        OneWay + the six control binders
├── bind_test.go
├── list.go        List[T] observable collection + Items binder
└── list_test.go
controls/stackpanel.go   + Clear() (detaches children via SetParent(nil))
cmd/fluo-gallery/main.go binding demo section
```

---

### Task 1: bind core + control binders

**Files:** `bind/bind.go`, `bind/bind_test.go`; modify `controls/stackpanel.go` (Clear — needed by Task 2 but lands here with tests since it's a controls change)

**Produces (exact):**
```go
package bind

// OneWay applies p's current value via apply, then re-applies on every change.
func OneWay[T comparable](p *core.Property[T], apply func(T)) (cancel func())

// Two-way binders. While bound, the binder OWNS the control's OnChanged slot;
// observe user edits through p. cancel() detaches both directions.
func Text(p *core.Property[string], tb *controls.TextBox) (cancel func())
func Checked(p *core.Property[bool], cb *controls.CheckBox) (cancel func())
func SwitchChecked(p *core.Property[bool], sw *controls.ToggleSwitch) (cancel func())
func ToggleChecked(p *core.Property[bool], tb *controls.ToggleButton) (cancel func())
func Value(p *core.Property[float32], s *controls.Slider) (cancel func())
func SelectedIndex(p *core.Property[int], cb *controls.ComboBox) (cancel func())
```
Mechanics (normative, identical shape per binder): on bind — control.SetX(p.Get()) [silent], control.OnChanged(func(v){ p.Set(v) }), sub := p.OnChange(func(_, v){ control.SetX(v) }) [silent → no echo]. cancel(): sub-cancel + control.OnChanged(nil); idempotent.

`StackPanel.Clear()`: removes all children, `core.SetParent(child, nil)` each, InvalidateMeasure, returns *StackPanel.

- [ ] TDD: echo-safety (bind Text; simulate user edit via the control's user path — router-driven typing or direct internal user-fire — assert p updated AND no infinite loop AND control not re-set redundantly (white-box: count SetText applications via a wrapper? simpler: assert final states)); model push updates control silently (OnChanged-observer on p fires once, control shows value); bind applies current value immediately; cancel detaches both directions (edits after cancel don't touch p; p.Set doesn't touch control) + idempotent cancel; one test per remaining binder (Checked/Switch/Toggle/Value/SelectedIndex) covering both directions; StackPanel.Clear detaches (children nil-parented, panel empty, measure-dirty). FAIL → implement → green.
- [ ] Suite+vet+gofmt; commit `feat(bind): one-way and two-way control binding`

### Task 2: observable List + Items

**Files:** `bind/list.go`, `bind/list_test.go`

**Produces (exact):**
```go
// List is an observable slice for collection binding. Not goroutine-safe.
type List[T any] struct{ /* items []T; subs map[int]func(); nextID int */ }
func NewList[T any](items ...T) *List[T]
func (l *List[T]) Len() int
func (l *List[T]) At(i int) T                  // panics out of range (fail-fast, documented)
func (l *List[T]) Add(items ...T)
func (l *List[T]) Insert(i int, item T)        // panics out of range
func (l *List[T]) RemoveAt(i int)              // panics out of range
func (l *List[T]) Set(i int, item T)
func (l *List[T]) Replace(items ...T)          // wholesale reset
func (l *List[T]) OnChanged(f func()) (cancel func()) // coarse-grained v0 (no change detail; rebuild-all consumers)

// Items binds list to panel: on ANY list change, panel.Clear() then Add(make(item))
// for each item, in order. v0 = full rebuild (virtualization arrives Phase 7).
func Items[T any](l *List[T], panel *controls.StackPanel, make func(item T, index int) core.Widget) (cancel func())
```
- [ ] TDD: List ops + notification counts (one per mutation; Replace once); Items initial population; Add/RemoveAt/Set rebuild reflects order (assert panel.Children() len + probe widget identity per index via make-closure recording); cancel stops rebuilds + is idempotent; empty list → empty panel. FAIL → implement → green.
- [ ] Suite+vet+gofmt; commit `feat(bind): observable List and Items collection binding`

### Task 3: gallery binding demo + docs

**Files:** `cmd/fluo-gallery/main.go`, `README.md`, `ROADMAP.md`

- New "Binding" rows in the controls section: (1) a `core.Property[string]` bound two-way to the existing TextBox AND one-way to a new TextBlock mirror (`bind.OneWay(p, func(s){ mirror.SetText(...) })`) — typing updates the mirror live; (2) the existing Slider/ProgressBar wiring REPLACED by a `core.Property[float32]` bound to the Slider (two-way) + OneWay to progress SetValue; (3) an ObservableList demo: small "Add" Button appends "Item N" to a `bind.List[string]` bound via `bind.Items` to a StackPanel below (rows = TextBlocks).
- Rebuild-on-theme-toggle: bindings from the OLD tree must be cancelled on rebuild (keep a `[]func()` of cancels; call all before buildUI) — this is the first real cancel consumer; do it properly.
- Live verify: launch, winshot `fluo-gallery-p6.png`, READ (binding rows present; mirror text shows the TextBox's initial/placeholder state; list section with Add button). Kill+confirm. Docs: ROADMAP tick all 3 Phase 6 boxes; README binding paragraph (conventions: silent setters, binder owns OnChanged, cancel discipline).
- [ ] Commit `feat(gallery): binding demos; complete Phase 6`

---

## Self-review notes
- ROADMAP coverage: one-way→1, two-way inputs→1, ItemsSource→2, (gallery/docs→3).
- The single-slot OnChanged ownership is documented in bind's package doc AND each binder.
- Type consistency: binder mechanics identical across all six; List.OnChanged coarse contract matches Items' rebuild-all.
