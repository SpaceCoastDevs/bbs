package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// --- Enums for screen state ---
type screenState int

const (
	splashScreen screenState = iota
	listScreen
	postDetailScreen // Added for post detail view
)

// --- Structs for Post Data ---
type PostMetadata struct {
	PostTitle   string    `yaml:"title"` // Renamed from Title to PostTitle
	Excerpt     string    `yaml:"excerpt"`
	PublishDate time.Time `yaml:"publishDate"`
	Category    string    `yaml:"category"`
	Tags        []string  `yaml:"tags"`
	Slug        string    `yaml:"slug"`
	Image       string    `yaml:"image"`
}

// Implement list.Item for PostMetadata
func (p PostMetadata) Title() string { return p.PostTitle } // Updated to use PostTitle
func (p PostMetadata) Description() string {
	desc := p.PublishDate.Format("2006-01-02")
	if p.Category != "" {
		desc += " | Cat: " + p.Category
	}
	if len(p.Tags) > 0 {
		desc += " | Tags: " + strings.Join(p.Tags, ", ")
	}
	return desc
}
func (p PostMetadata) FilterValue() string { return p.PostTitle + " " + p.Category + " " + strings.Join(p.Tags, " ") } // Updated to use PostTitle

// --- Messages ---
type tickMsg time.Time
type postsLoadedMsg struct {
	posts []PostMetadata
	err   error
}

// type gotPostsErrorMsg struct{ err error } // Not used in this simplified version

// --- Model ---
type model struct {
	currentScreen    screenState
	splashMessage    string
	flashMessage     string
	showFlashMessage bool
	width            int
	height           int
	postList         list.Model
	loadingPosts     bool
	postsError       error
	selectedPost     *PostMetadata // Store selected post for detail view
}

func initialModel() model {
	delegate := list.NewDefaultDelegate()

	adaptiveBg := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	// Main foreground for titles, matching the baseStyle's adaptiveForeground
	adaptiveFg := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	// Dimmed foreground for descriptions, common for secondary text in lists
	dimmedFg := lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}

	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(adaptiveFg).
		Background(adaptiveBg).
		Padding(0, 0, 0, 2) // Default padding

	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(dimmedFg).
		Background(adaptiveBg).
		Padding(0, 0, 0, 2) // Default padding

	// Selected items' styling from the screenshot seems acceptable (pink title, dark background for the row).
	// We'll leave delegate.Styles.SelectedTitle and delegate.Styles.SelectedDesc as default,
	// as they primarily define text color and a left border, not the row background.
	// The list component itself handles the selected row's background highlight.

	l := list.New([]list.Item{}, delegate, 0, 0) // Use the customized delegate
	l.Title = "Blog Posts"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true) // Pinkish title
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.Foreground(lipgloss.Color("240"))
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.Foreground(lipgloss.Color("240"))

	return model{
		currentScreen:    splashScreen,
		splashMessage:    "Welcome to Space Coast Devs",
		flashMessage:     "<Press Enter to Continue>",
		showFlashMessage: true,
		loadingPosts:     false,
		postList:         l,
	}
}

// --- GitHub Fetching Logic ---
const (
	repoOwner = "SpaceCoastDevs"
	repoName  = "space-coast.dev"
	repoAPIPath = "src/content/post"
	githubAPIContentsURLFormat = "https://api.github.com/repos/%s/%s/contents/%s"
)

// GitHubContent struct to unmarshal the JSON response from GitHub API
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

// Simplified representation of a file from GitHub API (not used in this simplified fetch)
// type GitHubFile struct {
// 	Name        string `json:"name"`
// 	Path        string `json:"path"`
// 	DownloadURL string `json:"download_url"` // URL to get raw content
// 	Type        string `json:"type"`         // "file" or "dir"
// }

// fetchPostsCmd simulates fetching and parsing posts.
// WARNING: This version uses a hardcoded list of file URLs.
// A real implementation would first query the GitHub API to get the list of .mdx files.
func fetchPostsCmd() tea.Cmd {
	return func() tea.Msg {
		var posts []PostMetadata
		client := &http.Client{Timeout: 20 * time.Second} // Increased timeout for multiple requests
		var firstError error

		// 1. Fetch directory listing from GitHub API
		apiURL := fmt.Sprintf(githubAPIContentsURLFormat, repoOwner, repoName, repoAPIPath)
		req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
		if err != nil {
			errMsg := fmt.Errorf("creating API request for %s: %w", apiURL, err)
			log.Println(errMsg)
			return postsLoadedMsg{posts: nil, err: errMsg}
		}

		apiResp, err := client.Do(req)
		if err != nil {
			errMsg := fmt.Errorf("fetching API %s: %w", apiURL, err)
			log.Println(errMsg)
			return postsLoadedMsg{posts: nil, err: errMsg}
		}
		defer apiResp.Body.Close()

		if apiResp.StatusCode != http.StatusOK {
			errMsg := fmt.Errorf("fetching API %s: status %s", apiURL, apiResp.Status)
			log.Println(errMsg)
			return postsLoadedMsg{posts: nil, err: errMsg}
		}

		apiBody, err := io.ReadAll(apiResp.Body) // Replaced ioutil.ReadAll with io.ReadAll
		if err != nil {
			errMsg := fmt.Errorf("reading API response body from %s: %w", apiURL, err)
			log.Println(errMsg)
			return postsLoadedMsg{posts: nil, err: errMsg}
		}

		var contents []GitHubContent
		err = json.Unmarshal(apiBody, &contents)
		if err != nil {
			errMsg := fmt.Errorf("unmarshalling API JSON from %s: %w", apiURL, err)
			log.Println(errMsg)
			return postsLoadedMsg{posts: nil, err: errMsg}
		}

		// 2. For each .mdx file, fetch its content and parse frontmatter
		for _, content := range contents {
			if content.Type == "file" && strings.HasSuffix(content.Name, ".mdx") && content.DownloadURL != "" {
				fileURL := content.DownloadURL
				fileReq, err := http.NewRequestWithContext(context.Background(), "GET", fileURL, nil)
				if err != nil {
					log.Printf("Error creating request for %s: %v", fileURL, err)
					if firstError == nil { firstError = fmt.Errorf("creating request for %s: %w", fileURL, err) }
					continue
				}

				resp, err := client.Do(fileReq)
				if err != nil {
					log.Printf("Error fetching %s: %v", fileURL, err)
					if firstError == nil { firstError = fmt.Errorf("fetching %s: %w", fileURL, err) }
					continue
				}

				if resp.StatusCode != http.StatusOK {
					log.Printf("Error fetching %s: status %s", fileURL, resp.Status)
					if firstError == nil { firstError = fmt.Errorf("fetching %s: status %s", fileURL, resp.Status) }
					resp.Body.Close()
					continue
				}

				body, err := io.ReadAll(resp.Body) // Replaced ioutil.ReadAll with io.ReadAll
				resp.Body.Close()
				if err != nil {
					log.Printf("Error reading body for %s: %v", fileURL, err)
					if firstError == nil { firstError = fmt.Errorf("reading body for %s: %w", fileURL, err) }
					continue
				}

				contentStr := string(body)
				parts := strings.SplitN(contentStr, "---", 3)
				if len(parts) < 3 {
					log.Printf("Could not find frontmatter in %s", fileURL)
					if firstError == nil { firstError = fmt.Errorf("no frontmatter in %s", fileURL) }
					continue
				}

				var meta PostMetadata
				err = yaml.Unmarshal([]byte(parts[1]), &meta)
				if err != nil {
					log.Printf("Error unmarshalling YAML for %s: %v", fileURL, err)
					if firstError == nil { firstError = fmt.Errorf("unmarshalling YAML for %s: %w", fileURL, err) }
					continue
				}
				posts = append(posts, meta)
			} else if content.Type == "file" && strings.HasSuffix(content.Name, ".mdx") {
				log.Printf("Skipping file %s as it has no download_url", content.Name)
			}
		}

		if len(posts) == 0 && firstError != nil {
			return postsLoadedMsg{posts: nil, err: fmt.Errorf("failed to load any posts, first error: %w", firstError)}
		}
		// If there were non-critical errors for some files but others loaded, we still return the loaded posts.
		// The individual errors are logged.
		return postsLoadedMsg{posts: posts, err: nil}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), tea.EnterAltScreen) // EnterAltScreen is good for list views
}

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.postList.SetWidth(msg.Width)
		m.postList.SetHeight(msg.Height)

	case tea.KeyMsg:
		switch m.currentScreen {
		case splashScreen:
			switch msg.String() {
			case "q", "esc", "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.currentScreen = listScreen
				m.loadingPosts = true
				m.postsError = nil
				m.postList.SetItems([]list.Item{}) // Clear previous items
				cmds = append(cmds, fetchPostsCmd())
			}
		case listScreen:
			if m.postList.FilterState() == list.Filtering {
				// Let the list handle keys during filtering
			} else {
				switch msg.String() {
				case "q", "esc":
					return m, tea.Quit
				case "b", "backspace":
					m.currentScreen = splashScreen
					m.showFlashMessage = true
					m.postsError = nil
					cmds = append(cmds, tick())
				case "enter":
					if item := m.postList.SelectedItem(); item != nil {
						if post, ok := item.(PostMetadata); ok {
							m.selectedPost = &post
							m.currentScreen = postDetailScreen
						}
					}
				}
			}
			var listCmd tea.Cmd
			m.postList, listCmd = m.postList.Update(msg)
			cmds = append(cmds, listCmd)
		case postDetailScreen:
			switch msg.String() {
			case "q", "esc", "b", "backspace":
				m.currentScreen = listScreen
				m.selectedPost = nil
			}
		}

	case tickMsg:
		if m.currentScreen == splashScreen {
			m.showFlashMessage = !m.showFlashMessage
			cmds = append(cmds, tick())
		}

	case postsLoadedMsg:
		m.loadingPosts = false
		if msg.err != nil {
			m.postsError = msg.err
			log.Printf("Error in postsLoadedMsg: %v", msg.err)
			m.postList.SetItems([]list.Item{}) // Clear list on error
		} else {
			items := make([]list.Item, len(msg.posts))
			for i, p := range msg.posts {
				items[i] = p // PostMetadata now implements list.Item
			}
			m.postList.SetItems(items)
			m.postsError = nil // Clear any previous error
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	adaptiveBackground := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	adaptiveForeground := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}

	baseStyle := lipgloss.NewStyle().
		Background(adaptiveBackground).
		Foreground(adaptiveForeground)

	switch m.currentScreen {
	case splashScreen:
		splashContainerStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center) // Removed .Copy()
		mainMessageStyle := lipgloss.NewStyle().Foreground(adaptiveForeground)
		mainMessageContent := mainMessageStyle.Render(m.splashMessage)
		flashingMessageContent := ""
		if m.showFlashMessage {
			flashStyle := lipgloss.NewStyle().Foreground(adaptiveForeground)
			flashingMessageContent = flashStyle.Render(m.flashMessage)
		}
		combinedContent := lipgloss.JoinVertical(lipgloss.Center,
			mainMessageContent,
			"",
			flashingMessageContent,
		)
		return splashContainerStyle.Render(combinedContent)

	case listScreen:
		if m.loadingPosts {
			loadingStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center)
			return loadingStyle.Render("Loading posts...")
		}
		if m.postsError != nil {
			errorStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center)
			content := fmt.Sprintf("Error loading posts: %v\n\n(Press 'b' to go back or 'q' to quit)", m.postsError)
			return errorStyle.Render(content)
		}
		// The list.Model.View() will handle rendering the list within the dimensions it was given.
		// We wrap it in a style that ensures the baseStyle (background, etc.) covers the whole screen area.
		listScreenContainerStyle := baseStyle.Width(m.width).Height(m.height)
		return listScreenContainerStyle.Render(m.postList.View())

	case postDetailScreen:
		if m.selectedPost == nil {
			return baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center).Render("No post selected. Press 'b' to go back.")
		}
		post := m.selectedPost
		detail := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(post.PostTitle),
			"",
			"Date: "+post.PublishDate.Format("2006-01-02"),
			"Category: "+post.Category,
			"Tags: "+strings.Join(post.Tags, ", "),
			"",
			post.Excerpt,
			"",
			"[Press 'b' or 'esc' to go back]",
		)
		return baseStyle.Width(m.width).Height(m.height).Padding(1,2).Render(detail)

	default:
		unknownScreenStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center) // Removed .Copy()
		return unknownScreenStyle.Render("Unknown screen")
	}
}

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatalf("could not open log file: %v", err)
	}
	defer f.Close()

	p := tea.NewProgram(initialModel())
	if _, errP := p.Run(); errP != nil { // Renamed err to errP to avoid conflict with f.Close() error
		log.Fatalf("Error running program: %v", errP)
	}
}

// itemDelegate is a custom list item delegate (example)
// We are using the default one for now, but this shows how you could customize rendering.
/*
type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 } // Or more if you render multiple lines
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	p, ok := listItem.(PostMetadata)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, p.Title())
	if m.Index() == index {
		// Style for selected item
		fmt.Fprint(w, lipgloss.NewStyle().Foreground(lipgloss.Color("202")).Render("> "+str))
	} else {
		// Style for normal item
		fmt.Fprint(w, str)
	}
	// You could add p.Description() on a new line here if Height() > 1
}
*/