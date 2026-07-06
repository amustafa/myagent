package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// flavorForm renders a schema as an interactive form. It uses a modal field
// editor: in field-nav mode ↑/↓ moves between visible fields; opening a field
// switches to a type-specific editor (text, enum picker, or multi-select) so
// key handling is never ambiguous.
type flavorForm struct {
	schema  *FlavorSchema
	values  map[string]any
	visible []int // indices into schema.Options that pass their showIf
	cursor  int   // 0..len(visible) — the extra slot is the Submit row
	mode    formMode
	sub     int             // sub-cursor within an editor
	custom  bool            // in enum editor: the "custom…" row is highlighted
	text    textinput.Model // reused for text/number/path/custom entry
	err     string
}

type formMode int

const (
	modeFieldNav formMode = iota
	modeEditText
	modeEditEnum
	modeEditMulti
)

// formResult signals what the caller should do after an update.
type formResult int

const (
	frNone formResult = iota
	frSubmit
	frCancel
)

func newFlavorForm(s *FlavorSchema, initial map[string]any) flavorForm {
	values := map[string]any{}
	for k, v := range s.defaults() {
		values[k] = v
	}
	for k, v := range initial { // pre-fill (e.g. editing an existing flavor)
		values[k] = v
	}
	ti := textinput.New()
	ti.Prompt = "› "
	ti.CharLimit = 2048
	f := flavorForm{schema: s, values: values, text: ti}
	f.recomputeVisible()
	return f
}

func (f *flavorForm) recomputeVisible() {
	f.visible = f.visible[:0]
	for i, o := range f.schema.Options {
		if o.visible(f.values) {
			f.visible = append(f.visible, i)
		}
	}
	if f.cursor > len(f.visible) {
		f.cursor = len(f.visible)
	}
}

func (f flavorForm) onSubmitRow() bool { return f.cursor == len(f.visible) }

func (f flavorForm) curOption() FlavorOption {
	return f.schema.Options[f.visible[f.cursor]]
}

func (f flavorForm) update(msg tea.KeyMsg) (flavorForm, tea.Cmd, formResult) {
	switch f.mode {
	case modeFieldNav:
		return f.updateNav(msg)
	case modeEditText:
		return f.updateText(msg)
	case modeEditEnum:
		return f.updateEnum(msg)
	case modeEditMulti:
		return f.updateMulti(msg)
	}
	return f, nil, frNone
}

func (f flavorForm) updateNav(msg tea.KeyMsg) (flavorForm, tea.Cmd, formResult) {
	f.err = ""
	switch msg.String() {
	case "up", "k":
		if f.cursor > 0 {
			f.cursor--
		}
	case "down", "j":
		if f.cursor < len(f.visible) {
			f.cursor++
		}
	case "esc":
		return f, nil, frCancel
	case " ", "enter":
		if f.onSubmitRow() {
			if err := f.schema.validateInput(f.values); err != nil {
				f.err = err.Error()
				return f, nil, frNone
			}
			return f, nil, frSubmit
		}
		return f.openEditor(msg.String())
	}
	return f, nil, frNone
}

// openEditor transitions into the right editor for the focused option.
func (f flavorForm) openEditor(key string) (flavorForm, tea.Cmd, formResult) {
	o := f.curOption()
	switch o.Type {
	case OptBool:
		if key == " " || key == "enter" { // toggle inline; no modal
			f.values[o.Key] = !asBool(f.values[o.Key])
			f.recomputeVisible()
		}
		return f, nil, frNone
	case OptEnumOne, OptEnumOrCustom:
		f.mode = modeEditEnum
		f.sub = indexOf(o.Enum, asString(f.values[o.Key]))
		f.custom = false
		if f.sub < 0 {
			f.sub = 0
			if o.Type == OptEnumOrCustom && asString(f.values[o.Key]) != "" {
				f.custom = true // current value isn't in the enum ⇒ it's a custom entry
			}
		}
		return f, nil, frNone
	case OptEnumSet, OptEnumList:
		f.mode = modeEditMulti
		f.sub = 0
		return f, nil, frNone
	default: // string, number, path
		f.mode = modeEditText
		f.text.SetValue(scalarString(f.values[o.Key]))
		f.text.CursorEnd()
		f.text.Placeholder = o.Placeholder
		return f, textinput.Blink, frNone
	}
}

func (f flavorForm) updateText(msg tea.KeyMsg) (flavorForm, tea.Cmd, formResult) {
	o := f.curOption()
	switch msg.Type {
	case tea.KeyEsc:
		f.mode = modeFieldNav
		return f, nil, frNone
	case tea.KeyTab:
		if o.Type == OptPath {
			completed, _ := completeDir(f.text.Value())
			f.text.SetValue(completed)
			f.text.CursorEnd()
		}
		return f, nil, frNone
	case tea.KeyEnter:
		val, err := coerceScalar(o, f.text.Value())
		if err != nil {
			f.err = err.Error()
			return f, nil, frNone
		}
		if err := o.validateValue(val); err != nil {
			f.err = err.Error()
			return f, nil, frNone
		}
		f.values[o.Key] = val
		f.err = ""
		f.mode = modeFieldNav
		f.recomputeVisible()
		return f, nil, frNone
	}
	var cmd tea.Cmd
	f.text, cmd = f.text.Update(msg)
	return f, cmd, frNone
}

func (f flavorForm) updateEnum(msg tea.KeyMsg) (flavorForm, tea.Cmd, formResult) {
	o := f.curOption()
	hasCustom := o.Type == OptEnumOrCustom
	rows := len(o.Enum)
	if hasCustom {
		rows++ // trailing custom row
	}
	switch msg.String() {
	case "up", "k":
		if f.sub > 0 {
			f.sub--
			f.custom = hasCustom && f.sub == len(o.Enum)
		}
	case "down", "j":
		if f.sub < rows-1 {
			f.sub++
			f.custom = hasCustom && f.sub == len(o.Enum)
		}
	case "esc":
		f.mode = modeFieldNav
	case "enter", " ":
		if f.custom { // open free-text entry for a custom value
			f.mode = modeEditText
			cur := asString(f.values[o.Key])
			if indexOf(o.Enum, cur) >= 0 {
				cur = ""
			}
			f.text.SetValue(cur)
			f.text.CursorEnd()
			return f, textinput.Blink, frNone
		}
		f.values[o.Key] = o.Enum[f.sub]
		f.mode = modeFieldNav
		f.recomputeVisible()
	}
	return f, nil, frNone
}

func (f flavorForm) updateMulti(msg tea.KeyMsg) (flavorForm, tea.Cmd, formResult) {
	o := f.curOption()
	sel := asStrings(f.values[o.Key])
	switch msg.String() {
	case "up", "k":
		if f.sub > 0 {
			f.sub--
		}
	case "down", "j":
		if f.sub < len(o.Enum)-1 {
			f.sub++
		}
	case " ": // toggle membership of the highlighted enum entry
		item := o.Enum[f.sub]
		if i := indexOf(sel, item); i >= 0 {
			sel = append(sel[:i], sel[i+1:]...)
		} else {
			sel = append(sel, item)
		}
		f.values[o.Key] = sel
	case "<", "[": // ordered list: move highlighted member earlier in order
		if o.Type == OptEnumList {
			sel = reorder(sel, o.Enum[f.sub], -1)
			f.values[o.Key] = sel
		}
	case ">", "]": // ordered list: move highlighted member later in order
		if o.Type == OptEnumList {
			sel = reorder(sel, o.Enum[f.sub], +1)
			f.values[o.Key] = sel
		}
	case "enter", "esc":
		f.mode = modeFieldNav
		f.recomputeVisible()
	}
	return f, nil, frNone
}

// ---- rendering --------------------------------------------------------------

func (f flavorForm) view() string {
	var b strings.Builder
	title := "Configure flavor"
	if f.schema.Flavor != "" {
		title = "Configure " + f.schema.Flavor
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	for vi, oi := range f.visible {
		o := f.schema.Options[oi]
		focused := vi == f.cursor && !f.onSubmitRow()
		cursor := "  "
		label := o.Label
		if focused {
			cursor = activeStyle.Render("▸ ")
			label = activeStyle.Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, label, dimStyle.Render(f.valueSummary(o))))
		if o.Help != "" && focused && f.mode == modeFieldNav {
			b.WriteString("      " + dimStyle.Render(o.Help) + "\n")
		}
		if focused && f.mode != modeFieldNav {
			b.WriteString(f.editorView(o))
		}
	}

	// Submit row.
	submit := "Submit"
	if f.onSubmitRow() {
		submit = activeStyle.Render("▸ Submit — name & generate")
	} else {
		submit = "  " + submit
	}
	b.WriteString("\n" + submit + "\n")

	if f.err != "" {
		b.WriteString(warnStyle.Render(f.err) + "\n")
	}
	b.WriteString(helpStyle.Render(f.helpLine()))
	return b.String() + "\n"
}

func (f flavorForm) helpLine() string {
	switch f.mode {
	case modeEditText:
		return "type · tab complete (path) · enter commit · esc cancel"
	case modeEditEnum:
		return "↑/↓ choose · enter select · esc cancel"
	case modeEditMulti:
		return "↑/↓ move · space toggle · </> reorder (ordered) · enter done"
	default:
		return "↑/↓ move · space/enter edit · esc cancel"
	}
}

func (f flavorForm) valueSummary(o FlavorOption) string {
	v := f.values[o.Key]
	switch {
	case o.isMulti():
		xs := asStrings(v)
		if len(xs) == 0 {
			return "(none)"
		}
		sep := ", "
		if o.Type == OptEnumList {
			sep = " > " // convey order
		}
		return strings.Join(xs, sep)
	case o.Type == OptBool:
		if asBool(v) {
			return "yes"
		}
		return "no"
	default:
		s := scalarString(v)
		if s == "" {
			return "(empty)"
		}
		return s
	}
}

func (f flavorForm) editorView(o FlavorOption) string {
	var b strings.Builder
	switch f.mode {
	case modeEditText:
		b.WriteString("      " + f.text.View() + "\n")
	case modeEditEnum:
		for i, e := range o.Enum {
			mark := "  "
			if i == f.sub && !f.custom {
				mark = activeStyle.Render("▸ ")
			}
			b.WriteString("      " + mark + e + "\n")
		}
		if o.Type == OptEnumOrCustom {
			mark := "  "
			if f.custom {
				mark = activeStyle.Render("▸ ")
			}
			b.WriteString("      " + mark + dimStyle.Render("✎ custom…") + "\n")
		}
	case modeEditMulti:
		sel := asStrings(f.values[o.Key])
		for i, e := range o.Enum {
			mark := "  "
			if i == f.sub {
				mark = activeStyle.Render("▸ ")
			}
			box := "[ ]"
			if pos := indexOf(sel, e); pos >= 0 {
				if o.Type == OptEnumList {
					box = okStyle.Render(fmt.Sprintf("[%d]", pos+1)) // show rank
				} else {
					box = okStyle.Render("[x]")
				}
			}
			b.WriteString("      " + mark + box + " " + e + "\n")
		}
	}
	return b.String()
}

// ---- value coercion helpers -------------------------------------------------

func coerceScalar(o FlavorOption, s string) (any, error) {
	s = strings.TrimSpace(s)
	if o.Type == OptNumber {
		if s == "" {
			return float64(0), nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a number", o.Label, s)
		}
		return f, nil
	}
	return s, nil
}

func scalarString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

func asBool(v any) bool     { b, _ := v.(bool); return b }
func asString(v any) string { s, _ := v.(string); return s }
func asStrings(v any) []string {
	if xs, ok := v.([]string); ok {
		return append([]string(nil), xs...)
	}
	return nil
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// reorder moves item one slot earlier (dir<0) or later (dir>0) within xs.
func reorder(xs []string, item string, dir int) []string {
	i := indexOf(xs, item)
	if i < 0 {
		return xs
	}
	j := i + dir
	if j < 0 || j >= len(xs) {
		return xs
	}
	xs[i], xs[j] = xs[j], xs[i]
	return xs
}
