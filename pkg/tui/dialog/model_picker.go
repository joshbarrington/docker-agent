package dialog

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/docker/docker-agent/pkg/model/provider"
	"github.com/docker/docker-agent/pkg/runtime"
	"github.com/docker/docker-agent/pkg/tui/components/scrollview"
	"github.com/docker/docker-agent/pkg/tui/components/toolcommon"
	"github.com/docker/docker-agent/pkg/tui/core"
	"github.com/docker/docker-agent/pkg/tui/core/layout"
	"github.com/docker/docker-agent/pkg/tui/messages"
	"github.com/docker/docker-agent/pkg/tui/styles"
)

// modelPickerDialog is a dialog for selecting a model for the current agent.
type modelPickerDialog struct {
	BaseDialog

	textInput  textinput.Model
	models     []runtime.ModelChoice
	filtered   []runtime.ModelChoice
	selected   int
	keyMap     commandPaletteKeyMap
	errMsg     string // validation error message
	scrollview *scrollview.Model

	// Double-click detection
	lastClickTime  time.Time
	lastClickIndex int
}

// NewModelPickerDialog creates a new model picker dialog.
func NewModelPickerDialog(models []runtime.ModelChoice) Dialog {
	ti := textinput.New()
	ti.Placeholder = "Type to search or enter custom model (provider/model)…"
	ti.Focus()
	ti.CharLimit = 100
	ti.SetWidth(50)

	// Sort models: config first, then catalog, then custom. Within each section: current first, then default, then alphabetically
	sortedModels := make([]runtime.ModelChoice, len(models))
	copy(sortedModels, models)
	slices.SortFunc(sortedModels, func(a, b runtime.ModelChoice) int {
		// Get section priority: config (0) < catalog (1) < custom (2)
		getPriority := func(m runtime.ModelChoice) int {
			if m.IsCustom {
				return 2
			}
			if m.IsCatalog {
				return 1
			}
			return 0
		}
		pa, pb := getPriority(a), getPriority(b)
		if pa != pb {
			return cmp.Compare(pa, pb)
		}
		// Within each section: current model first
		if a.IsCurrent != b.IsCurrent {
			if a.IsCurrent {
				return -1
			}
			return 1
		}
		// Then default model
		if a.IsDefault != b.IsDefault {
			if a.IsDefault {
				return -1
			}
			return 1
		}
		// Then alphabetically by name
		return cmp.Compare(a.Name, b.Name)
	})

	d := &modelPickerDialog{
		textInput:  ti,
		models:     sortedModels,
		keyMap:     defaultCommandPaletteKeyMap(),
		scrollview: scrollview.New(scrollview.WithReserveScrollbarSpace(true)),
	}
	d.filterModels()
	return d
}

func (d *modelPickerDialog) Init() tea.Cmd {
	return textinput.Blink
}

func (d *modelPickerDialog) Update(msg tea.Msg) (layout.Model, tea.Cmd) {
	// Scrollview handles mouse scrollbar, wheel, and pgup/pgdn/home/end
	if handled, cmd := d.scrollview.Update(msg); handled {
		return d, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmd := d.SetSize(msg.Width, msg.Height)
		return d, cmd

	case tea.PasteMsg:
		var cmd tea.Cmd
		d.textInput, cmd = d.textInput.Update(msg)
		d.filterModels()
		d.errMsg = ""
		return d, cmd

	case tea.MouseClickMsg:
		// Scrollbar clicks handled above; this handles list item clicks
		if msg.Button == tea.MouseLeft {
			if modelIdx := d.mouseYToModelIndex(msg.Y); modelIdx >= 0 {
				now := time.Now()
				if modelIdx == d.lastClickIndex && now.Sub(d.lastClickTime) < styles.DoubleClickThreshold {
					d.selected = modelIdx
					d.lastClickTime = time.Time{}
					cmd := d.handleSelection()
					return d, cmd
				}
				d.selected = modelIdx
				d.lastClickTime = now
				d.lastClickIndex = modelIdx
			}
		}
		return d, nil

	case tea.KeyPressMsg:
		if cmd := HandleQuit(msg); cmd != nil {
			return d, cmd
		}

		switch {
		case key.Matches(msg, d.keyMap.Escape):
			return d, core.CmdHandler(CloseDialogMsg{})

		case key.Matches(msg, d.keyMap.Up):
			if d.selected > 0 {
				d.selected--
				d.scrollview.EnsureLineVisible(d.findSelectedLine(nil))
			}
			return d, nil

		case key.Matches(msg, d.keyMap.Down):
			if d.selected < len(d.filtered)-1 {
				d.selected++
				d.scrollview.EnsureLineVisible(d.findSelectedLine(nil))
			}
			return d, nil

		case key.Matches(msg, d.keyMap.Enter):
			cmd := d.handleSelection()
			return d, cmd

		default:
			var cmd tea.Cmd
			d.textInput, cmd = d.textInput.Update(msg)
			d.filterModels()
			d.errMsg = ""
			return d, cmd
		}
	}

	return d, nil
}

// mouseYToModelIndex converts a mouse Y position to a model index.
// Returns -1 if the position is not on a model (e.g., on a separator or outside the list).
func (d *modelPickerDialog) mouseYToModelIndex(y int) int {
	dialogRow, _ := d.Position()
	maxItems := d.scrollview.VisibleHeight()

	listStartY := dialogRow + pickerListStartOffset
	listEndY := listStartY + maxItems

	// Check if Y is within the model list area
	if y < listStartY || y >= listEndY {
		return -1
	}

	// Calculate which line in the visible area was clicked
	lineInView := y - listStartY
	scrollOffset := d.scrollview.ScrollOffset()

	// Calculate the actual line index in allModelLines
	actualLine := scrollOffset + lineInView

	// Now we need to map the line back to a model index, accounting for separators
	return d.lineToModelIndex(actualLine)
}

// lineToModelIndex converts a line index (in allModelLines) to a model index.
// Returns -1 if the line is a separator.
func (d *modelPickerDialog) lineToModelIndex(lineIdx int) int {
	// Pre-compute model type flags (same logic as View)
	hasConfigModels := false
	hasCatalogModels := false
	for _, m := range d.filtered {
		switch {
		case m.IsCustom:
			// Custom models don't affect separator logic for config/catalog
		case m.IsCatalog:
			hasCatalogModels = true
		default:
			hasConfigModels = true
		}
	}

	// Walk through the models, counting lines including separators
	currentLine := 0
	catalogSeparatorShown := false
	customSeparatorShown := false

	for i, model := range d.filtered {
		// Check if separator would be added before this model
		if model.IsCatalog && !catalogSeparatorShown && !model.IsCustom {
			if hasConfigModels {
				if currentLine == lineIdx {
					return -1 // Clicked on separator
				}
				currentLine++
			}
			catalogSeparatorShown = true
		}

		if model.IsCustom && !customSeparatorShown {
			if hasConfigModels || hasCatalogModels {
				if currentLine == lineIdx {
					return -1 // Clicked on separator
				}
				currentLine++
			}
			customSeparatorShown = true
		}

		if currentLine == lineIdx {
			return i // Found the model at this line
		}
		currentLine++
	}

	return -1 // Line index out of range
}

func (d *modelPickerDialog) handleSelection() tea.Cmd {
	query := strings.TrimSpace(d.textInput.Value())

	// If user typed something that looks like a custom model (contains /), validate and use it
	if strings.Contains(query, "/") {
		if err := validateCustomModelSpec(query); err != nil {
			d.errMsg = err.Error()
			return nil
		}
		return tea.Sequence(
			core.CmdHandler(CloseDialogMsg{}),
			core.CmdHandler(messages.ChangeModelMsg{ModelRef: query}),
		)
	}

	// Otherwise, use the selected item from the filtered list
	if d.selected >= 0 && d.selected < len(d.filtered) {
		selected := d.filtered[d.selected]
		// If selecting the default model, send empty ref to clear the override
		modelRef := selected.Ref
		if selected.IsDefault {
			modelRef = ""
		}
		return tea.Sequence(
			core.CmdHandler(CloseDialogMsg{}),
			core.CmdHandler(messages.ChangeModelMsg{ModelRef: modelRef}),
		)
	}

	return nil
}

// validateCustomModelSpec validates a custom model specification entered by the user.
// It checks that each provider/model pair is properly formatted and uses a supported provider.
func validateCustomModelSpec(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}

	// Handle alloy specs (comma-separated)
	parts := strings.SplitSeq(spec, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		providerName, modelName, ok := strings.Cut(part, "/")
		if !ok {
			return errors.New("invalid format: expected 'provider/model'")
		}

		providerName = strings.TrimSpace(providerName)
		modelName = strings.TrimSpace(modelName)

		if providerName == "" {
			return fmt.Errorf("provider name cannot be empty (got '/%s')", modelName)
		}
		if modelName == "" {
			return fmt.Errorf("model name cannot be empty (got '%s/')", providerName)
		}

		if !provider.IsKnownProvider(providerName) {
			return fmt.Errorf("unknown provider '%s'. Supported: %s",
				providerName, strings.Join(provider.AllProviders(), ", "))
		}
	}

	return nil
}

func (d *modelPickerDialog) filterModels() {
	query := strings.ToLower(strings.TrimSpace(d.textInput.Value()))

	// If query contains "/", show "Custom" option as well as matches
	isCustomQuery := strings.Contains(query, "/")

	d.filtered = nil
	for _, model := range d.models {
		if query == "" {
			d.filtered = append(d.filtered, model)
			continue
		}

		// Match against name, provider, and model
		searchText := strings.ToLower(model.Name + " " + model.Provider + " " + model.Model)
		if strings.Contains(searchText, query) {
			d.filtered = append(d.filtered, model)
		}
	}

	// If query looks like a custom model spec and we have no exact match, show it as an option
	if isCustomQuery && len(d.filtered) == 0 {
		d.filtered = append(d.filtered, runtime.ModelChoice{
			Name: "Custom: " + query,
			Ref:  query,
		})
	}

	if d.selected >= len(d.filtered) {
		d.selected = max(0, len(d.filtered)-1)
	}
	// Reset scroll when filtering
	d.scrollview.SetScrollOffset(0)
}

// Model picker dialog dimension constants
const (
	// pickerWidthPercent is the percentage of screen width to use for the dialog
	pickerWidthPercent = 80
	// pickerMinWidth is the minimum width of the dialog
	pickerMinWidth = 50
	// pickerMaxWidth is the maximum width of the dialog
	pickerMaxWidth = 100
	// pickerHeightPercent is the percentage of screen height to use for the dialog
	pickerHeightPercent = 70
	// pickerMaxHeight is the maximum height of the dialog
	pickerMaxHeight = 150

	// pickerDialogPadding is the horizontal padding inside the dialog border (2 on each side + border)
	pickerDialogPadding = 6

	// Column widths for the per-row stats. Values are right-aligned in their
	// own column so the list reads like a table.
	pickerInputColWidth   = 10
	pickerOutputColWidth  = 10
	pickerContextColWidth = 8

	// pickerDetailsLines is the number of lines reserved for the model
	// details panel rendered below the model list.
	pickerDetailsLines = 4

	// pickerListVerticalOverhead is the number of rows used by dialog chrome:
	// title(1) + space(1) + input(1) + separator(1) + column header(1) +
	// details separator(1) + details (pickerDetailsLines) + space at bottom(1) +
	// help keys(1) + borders/padding(2) = 10 + pickerDetailsLines
	pickerListVerticalOverhead = 10 + pickerDetailsLines

	// pickerListStartOffset is the Y offset from dialog top to where the model list starts:
	// border(1) + padding(1) + title(1) + space(1) + input(1) + separator(1) +
	// column header(1) = 7
	pickerListStartOffset = 7

	// pickerDetailsLabelWidth is the column width for the labels in the
	// details panel ("Reference", "Pricing", "Limits", "Modalities").
	pickerDetailsLabelWidth = 12

	// catalogSeparatorLabel is the text for the catalog section separator
	catalogSeparatorLabel = "── Other models "
	// customSeparatorLabel is the text for the custom models section separator
	customSeparatorLabel = "── Custom models "
)

func (d *modelPickerDialog) dialogSize() (dialogWidth, maxHeight, contentWidth int) {
	dialogWidth = max(min(d.Width()*pickerWidthPercent/100, pickerMaxWidth), pickerMinWidth)
	maxHeight = min(d.Height()*pickerHeightPercent/100, pickerMaxHeight)
	contentWidth = dialogWidth - pickerDialogPadding - d.scrollview.ReservedCols()
	return dialogWidth, maxHeight, contentWidth
}

// SetSize sets the dialog dimensions and configures the scrollview.
func (d *modelPickerDialog) SetSize(width, height int) tea.Cmd {
	cmd := d.BaseDialog.SetSize(width, height)
	_, maxHeight, contentWidth := d.dialogSize()
	regionWidth := contentWidth + d.scrollview.ReservedCols()
	visLines := max(1, maxHeight-pickerListVerticalOverhead)
	d.scrollview.SetSize(regionWidth, visLines)
	return cmd
}

func (d *modelPickerDialog) View() string {
	dialogWidth, _, contentWidth := d.dialogSize()

	d.textInput.SetWidth(contentWidth)

	// Build all model lines first to calculate total height
	var allModelLines []string
	catalogSeparatorShown := false
	customSeparatorShown := false

	// Pre-compute if we have different model types to decide on separators
	hasConfigModels := false
	hasCatalogModels := false
	for _, m := range d.filtered {
		switch {
		case m.IsCustom:
			// Custom models don't affect separator logic for config/catalog
		case m.IsCatalog:
			hasCatalogModels = true
		default:
			hasConfigModels = true
		}
	}

	for i, model := range d.filtered {
		// Add separator before first catalog model (if there are config models anywhere in the list)
		if model.IsCatalog && !catalogSeparatorShown && !model.IsCustom {
			if hasConfigModels {
				separatorLine := styles.MutedStyle.Render(catalogSeparatorLabel + strings.Repeat("─", max(0, contentWidth-len(catalogSeparatorLabel)-2)))
				allModelLines = append(allModelLines, separatorLine)
			}
			catalogSeparatorShown = true
		}

		// Add separator before first custom model (if there are other models anywhere in the list)
		if model.IsCustom && !customSeparatorShown {
			if hasConfigModels || hasCatalogModels {
				separatorLine := styles.MutedStyle.Render(customSeparatorLabel + strings.Repeat("─", max(0, contentWidth-len(customSeparatorLabel)-2)))
				allModelLines = append(allModelLines, separatorLine)
			}
			customSeparatorShown = true
		}

		allModelLines = append(allModelLines, d.renderModel(model, i == d.selected, contentWidth))
	}

	regionWidth := contentWidth + d.scrollview.ReservedCols()

	// Set scrollview position for mouse hit-testing (auto-computed from dialog position)
	dialogRow, dialogCol := d.Position()
	d.scrollview.SetPosition(dialogCol+3, dialogRow+pickerListStartOffset)

	d.scrollview.SetContent(allModelLines, len(allModelLines))

	var scrollableContent string
	if len(d.filtered) == 0 {
		visLines := d.scrollview.VisibleHeight()
		emptyLines := []string{"", styles.DialogContentStyle.
			Italic(true).Align(lipgloss.Center).Width(contentWidth).
			Render("No models found")}
		for len(emptyLines) < visLines {
			emptyLines = append(emptyLines, "")
		}
		scrollableContent = d.scrollview.ViewWithLines(emptyLines)
	} else {
		scrollableContent = d.scrollview.View()
	}

	contentBuilder := NewContent(regionWidth).
		AddTitle("Select Model").
		AddSpace().
		AddContent(d.textInput.View())

	// Show error message if present
	if d.errMsg != "" {
		contentBuilder.AddContent(styles.ErrorStyle.Render("⚠ " + d.errMsg))
	}

	content := contentBuilder.
		AddSeparator().
		AddContent(d.renderColumnHeader(contentWidth)).
		AddContent(scrollableContent).
		AddSeparator().
		AddContent(d.renderDetails(contentWidth)).
		AddSpace().
		AddHelpKeys("↑/↓", "navigate", "enter", "select", "esc", "cancel").
		Build()

	return styles.DialogStyle.Width(dialogWidth).Render(content)
}

// findSelectedLine returns the line index in allModelLines that corresponds to the selected model.
// This accounts for separator lines that are inserted before catalog and custom sections.
func (d *modelPickerDialog) findSelectedLine(allModelLines []string) int {
	if d.selected < 0 || d.selected >= len(d.filtered) {
		return 0
	}

	// Pre-compute model type flags (same logic as View)
	hasConfigModels := false
	hasCatalogModels := false
	for _, m := range d.filtered {
		switch {
		case m.IsCustom:
			// Custom models don't affect separator logic for config/catalog
		case m.IsCatalog:
			hasCatalogModels = true
		default:
			hasConfigModels = true
		}
	}

	// Count lines before the selected model, including separators
	lineIndex := 0
	catalogSeparatorShown := false
	customSeparatorShown := false

	for i := range d.selected + 1 {
		model := d.filtered[i]

		// Check if separator was added before this model
		if model.IsCatalog && !catalogSeparatorShown && !model.IsCustom {
			if hasConfigModels && i <= d.selected {
				lineIndex++ // Count the separator
			}
			catalogSeparatorShown = true
		}

		if model.IsCustom && !customSeparatorShown {
			if (hasConfigModels || hasCatalogModels) && i <= d.selected {
				lineIndex++ // Count the separator
			}
			customSeparatorShown = true
		}

		if i == d.selected {
			return lineIndex
		}
		lineIndex++
	}

	return min(lineIndex, len(allModelLines)-1)
}

// pickerRowPalette is the set of styles used to render one row of the
// model list. Selection inverts the foreground/background colours of
// every visible element so the row reads as a single highlighted band.
type pickerRowPalette struct {
	name     lipgloss.Style
	desc     lipgloss.Style
	alloy    lipgloss.Style
	defBadge lipgloss.Style
	current  lipgloss.Style
	stats    lipgloss.Style
	missing  lipgloss.Style
}

func pickerRowStyles(selected bool) pickerRowPalette {
	p := pickerRowPalette{
		name:     styles.PaletteUnselectedActionStyle,
		desc:     styles.PaletteUnselectedDescStyle,
		alloy:    styles.BadgeAlloyStyle,
		defBadge: styles.BadgeDefaultStyle,
		current:  styles.BadgeCurrentStyle,
		stats:    styles.SecondaryStyle,
		missing:  styles.MutedStyle,
	}
	if !selected {
		return p
	}
	p.name = styles.PaletteSelectedActionStyle
	p.desc = styles.PaletteSelectedDescStyle
	p.alloy = p.alloy.Background(styles.MobyBlue)
	p.defBadge = p.defBadge.Background(styles.MobyBlue)
	p.current = p.current.Background(styles.MobyBlue)
	// Reuse the description style so the cells share the selection band.
	p.stats = p.desc
	p.missing = p.desc.Italic(true)
	return p
}

func (d *modelPickerDialog) renderModel(model runtime.ModelChoice, selected bool, maxWidth int) string {
	p := pickerRowStyles(selected)
	nameWidth := pickerNameColWidth(maxWidth)
	return renderRowName(model, nameWidth, p) + renderRowStats(model, p)
}

// pickerNameColWidth returns the width allotted to the name column for
// a given total content width.
func pickerNameColWidth(maxWidth int) int {
	return max(1, maxWidth-pickerInputColWidth-pickerOutputColWidth-pickerContextColWidth)
}

// renderRowName renders the model name and any badges, padded to width.
func renderRowName(model runtime.ModelChoice, width int, p pickerRowPalette) string {
	badges, badgeWidth := renderRowBadges(model, p)

	nameMax := max(1, width-badgeWidth)
	displayName := model.Name
	if lipgloss.Width(displayName) > nameMax {
		displayName = toolcommon.TruncateText(displayName, nameMax)
	}

	name := p.name.Render(displayName) + badges
	padding := max(0, width-lipgloss.Width(name))
	return name + p.desc.Render(strings.Repeat(" ", padding))
}

// renderRowBadges returns the rendered badge segment plus its width.
func renderRowBadges(model runtime.ModelChoice, p pickerRowPalette) (string, int) {
	var (
		text  string
		width int
	)
	add := func(label string, style lipgloss.Style) {
		text += style.Render(label)
		width += lipgloss.Width(label)
	}
	if isAlloyModel(model) {
		add(" (alloy)", p.alloy)
	}
	switch {
	case model.IsCurrent:
		add(" (current)", p.current)
	case model.IsDefault:
		add(" (default)", p.defBadge)
	}
	return text, width
}

// renderRowStats renders the three right-aligned stats columns.
func renderRowStats(model runtime.ModelChoice, p pickerRowPalette) string {
	return renderStatsCell(formatCostPerMillion(model.InputCost), pickerInputColWidth, p, model.InputCost > 0) +
		renderStatsCell(formatCostPerMillion(model.OutputCost), pickerOutputColWidth, p, model.OutputCost > 0) +
		renderStatsCell(formatContextCell(model.ContextLimit), pickerContextColWidth, p, model.ContextLimit > 0)
}

// renderStatsCell right-aligns value in a fixed-width column. Missing
// values fade by using the palette's missing style.
func renderStatsCell(value string, width int, p pickerRowPalette, present bool) string {
	padding := max(0, width-lipgloss.Width(value))
	pad := p.stats.Render(strings.Repeat(" ", padding))
	valueStyle := p.stats
	if !present {
		valueStyle = p.missing
	}
	return pad + valueStyle.Render(value)
}

// isAlloyModel returns true when the model is an alloy spec (no
// provider, comma-separated provider/model list in Model).
func isAlloyModel(model runtime.ModelChoice) bool {
	return model.Provider == "" && strings.Contains(model.Model, ",")
}

// renderColumnHeader renders the static header above the model list,
// labelling the per-row stats columns.
func (d *modelPickerDialog) renderColumnHeader(maxWidth int) string {
	header := strings.Repeat(" ", pickerNameColWidth(maxWidth)) +
		rightAlign("Input/1M", pickerInputColWidth) +
		rightAlign("Output/1M", pickerOutputColWidth) +
		rightAlign("Context", pickerContextColWidth)
	return styles.MutedStyle.Render(header)
}

// rightAlign returns s padded with leading spaces so its rendered width
// equals width. Strings already wider than width are returned unchanged.
func rightAlign(s string, width int) string {
	padding := width - lipgloss.Width(s)
	if padding <= 0 {
		return s
	}
	return strings.Repeat(" ", padding) + s
}

// leftPad returns s padded with trailing spaces to width. Strings already
// wider than width are returned unchanged.
func leftPad(s string, width int) string {
	padding := width - lipgloss.Width(s)
	if padding <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padding)
}

// formatContextCell formats a context window size for the table column.
// Returns an em-dash placeholder when the size is unknown.
func formatContextCell(tokens int) string {
	if tokens <= 0 {
		return "—"
	}
	return formatTokenCount(int64(tokens))
}

// formatCostPerMillion renders a USD-per-million-tokens price using a
// compact representation. Values <= 0 render as an em-dash; sub-cent
// values keep four decimals so they don't collapse to "$0.00";
// sub-dollar values keep two decimals; larger values trim trailing
// zeros (e.g., $3 instead of $3.00).
func formatCostPerMillion(cost float64) string {
	switch {
	case cost <= 0:
		return "—"
	case cost < 0.01:
		return fmt.Sprintf("$%.4f", cost)
	case cost < 1:
		return fmt.Sprintf("$%.2f", cost)
	}
	s := strconv.FormatFloat(cost, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return "$" + s
}

// modelReference returns the technical "provider/model" reference for a
// model choice, suitable for the details panel.
func modelReference(model runtime.ModelChoice) string {
	switch {
	case model.IsCustom:
		return model.Ref
	case isAlloyModel(model):
		return model.Model
	case model.Provider != "" && model.Model != "":
		return model.Provider + "/" + model.Model
	default:
		return model.Ref
	}
}

// detailsStyles bundles the styles used by the details panel.
type detailsStyles struct {
	label lipgloss.Style
	value lipgloss.Style
	muted lipgloss.Style
}

func newDetailsStyles() detailsStyles {
	return detailsStyles{
		label: styles.SecondaryStyle.Bold(true),
		value: styles.BaseStyle,
		muted: styles.MutedStyle.Italic(true),
	}
}

// renderDetails returns the details panel for the currently-selected
// model. It always renders pickerDetailsLines lines so the dialog has a
// stable height.
func (d *modelPickerDialog) renderDetails(width int) string {
	s := newDetailsStyles()

	var lines []string
	if d.selected >= 0 && d.selected < len(d.filtered) {
		lines = formatDetailsLines(d.filtered[d.selected], s)
	} else {
		lines = []string{s.muted.Render("No model selected")}
	}

	// Pad to a stable height so the dialog doesn't change size.
	for len(lines) < pickerDetailsLines {
		lines = append(lines, "")
	}
	// Truncate any line that would wrap.
	for i, l := range lines {
		if lipgloss.Width(l) > width {
			lines[i] = toolcommon.TruncateText(l, width)
		}
	}
	return strings.Join(lines[:pickerDetailsLines], "\n")
}

// formatDetailsLines builds the four labelled rows shown for a model.
func formatDetailsLines(model runtime.ModelChoice, s detailsStyles) []string {
	row := func(label, value string) string {
		return s.label.Render(leftPad(label, pickerDetailsLabelWidth)) + value
	}

	ref := s.value.Render(modelReference(model))
	if model.Family != "" && !strings.EqualFold(model.Family, model.Provider) {
		ref += s.muted.Render(" · " + model.Family + " family")
	}

	return []string{
		row("Reference", ref),
		row("Pricing", formatPricingRow(model, s)),
		row("Limits", formatLimitsRow(model, s)),
		row("Modalities", formatModalitiesRow(model, s)),
	}
}

// formatPricingRow renders the pricing line of the details panel.
func formatPricingRow(model runtime.ModelChoice, s detailsStyles) string {
	var parts []string
	if model.InputCost > 0 || model.OutputCost > 0 {
		parts = append(parts,
			s.value.Render(formatCostPerMillion(model.InputCost)+" in"),
			s.value.Render(formatCostPerMillion(model.OutputCost)+" out"),
		)
	}
	if model.CacheReadCost > 0 {
		parts = append(parts, s.value.Render(formatCostPerMillion(model.CacheReadCost)+" cache read"))
	}
	if model.CacheWriteCost > 0 {
		parts = append(parts, s.value.Render(formatCostPerMillion(model.CacheWriteCost)+" cache write"))
	}
	if len(parts) == 0 {
		return s.muted.Render("unavailable")
	}
	parts = append(parts, s.muted.Render("per 1M tokens"))
	return strings.Join(parts, s.muted.Render(" · "))
}

// formatLimitsRow renders the limits line of the details panel.
func formatLimitsRow(model runtime.ModelChoice, s detailsStyles) string {
	var parts []string
	if model.ContextLimit > 0 {
		parts = append(parts, s.value.Render(formatTokenCount(int64(model.ContextLimit))+" context window"))
	}
	if model.OutputLimit > 0 {
		parts = append(parts, s.value.Render(formatTokenCount(model.OutputLimit)+" max output"))
	}
	if len(parts) == 0 {
		return s.muted.Render("unavailable")
	}
	return strings.Join(parts, s.muted.Render(" · "))
}

// formatModalitiesRow renders the modalities line of the details panel.
func formatModalitiesRow(model runtime.ModelChoice, s detailsStyles) string {
	if len(model.InputModalities) == 0 && len(model.OutputModalities) == 0 {
		return s.muted.Render("unavailable")
	}
	in := joinOrDash(model.InputModalities)
	out := joinOrDash(model.OutputModalities)
	return s.value.Render(in) + s.muted.Render(" → ") + s.value.Render(out)
}

// joinOrDash returns the comma-joined list, or an em-dash when empty.
func joinOrDash(parts []string) string {
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, ", ")
}

func (d *modelPickerDialog) Position() (row, col int) {
	dialogWidth, maxHeight, _ := d.dialogSize()
	return CenterPosition(d.Width(), d.Height(), dialogWidth, maxHeight)
}
