package main

// A simple program that opens the alternate screen buffer then counts down
// from 5 and then exits.

import (
	"log"
	"time" // Import time package

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time // Re-add tickMsg

type model struct {
	message          string
	flashMessage     string
	showFlashMessage bool
	width            int
	height           int
}

func initialModel() model {
	return model{
		message:          "Welcome to Space Coast Devs",
		flashMessage:     "<Click Enter to Continue>",
		showFlashMessage: true, // Start with it visible
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func (m model) Init() tea.Cmd {
	return tick() // Start the flashing timer
}

// tick returns a tea.Cmd that sends a tickMsg after an interval.
func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg { // Flash every 500ms
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c", "enter": // Add "enter" to quit keys
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg: // Handle the tick for flashing
		m.showFlashMessage = !m.showFlashMessage
		return m, tick() // Continue ticking
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Define adaptive colors
	adaptiveBackground := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	adaptiveForeground := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}

	// Main message style (no specific alignment here, will be part of a larger centered block)
	mainMessageStyle := lipgloss.NewStyle().
		Foreground(adaptiveForeground)

	mainMessageContent := mainMessageStyle.Render(m.message)

	// Flashing message content (conditionally rendered)
	flashingMessageContent := ""
	if m.showFlashMessage {
		// Style for the flashing message, could be different if needed
		flashStyle := lipgloss.NewStyle().Foreground(adaptiveForeground) // Inherits background from container
		flashingMessageContent = flashStyle.Render(m.flashMessage)
	}

	// Join the main message and the flashing message vertically, with a blank line in between.
	// Align the content of this block to the center horizontally
	combinedContent := lipgloss.JoinVertical(lipgloss.Center,
		mainMessageContent,
		"", // This adds a blank line
		flashingMessageContent, // This will be an empty string when not shown, preserving layout
	)

	// Overall style for centering the combined block on the screen
	screenStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center). // Center the block itself
		Background(adaptiveBackground)

	return screenStyle.Render(combinedContent)
}