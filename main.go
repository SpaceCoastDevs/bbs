package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp" // Added regexp import
	"sort"
	"strconv" // For footnote check
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport" // Added viewport import
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour" // Added glamour import
	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/lipgloss"
	ssh "github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"gopkg.in/yaml.v3"
)

// --- Enums for screen state ---
type screenState int

const (
	splashScreen screenState = iota
	listScreen
)

// --- Structs for Post Data ---
type PostMetadata struct {
	PostTitle   string    `yaml:"title"`
	Excerpt     string    `yaml:"excerpt"`
	PublishDate time.Time `yaml:"publishDate"`
	Category    string    `yaml:"category"`
	Tags        []string  `yaml:"tags"`
	Slug        string    `yaml:"slug"`
	Image       string    `yaml:"image"`
	Content     string    // Added to store the full post content
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
	selectedPost     *PostMetadata
	viewport         viewport.Model // Added viewport for post content
	ready            bool           // For viewport initialization
}

func initialModel() model {
	// ... (existing list initialization) ...
	delegate := list.NewDefaultDelegate()

	adaptiveBg := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	adaptiveFg := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	dimmedFg := lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}

	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(adaptiveFg).
		Background(adaptiveBg).
		Padding(0, 0, 0, 2)

	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(dimmedFg).
		Background(adaptiveBg).
		Padding(0, 0, 0, 2)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Blog Posts"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.Foreground(lipgloss.Color("240"))
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.Foreground(lipgloss.Color("240"))

	// Viewport setup - will be fully configured when a post is selected
	vp := viewport.New(0,0) // Initial size, will be updated

	return model{
		currentScreen:    splashScreen,
		splashMessage:    "Welcome to Space Coast Devs",
		flashMessage:     "<Press Enter to Continue>",
		showFlashMessage: true,
		loadingPosts:     false,
		postList:         l,
		viewport:         vp,
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
				meta.Content = strings.TrimSpace(parts[2]) // Store the main content
				posts = append(posts, meta)
			} else if content.Type == "file" && strings.HasSuffix(content.Name, ".mdx") {
				log.Printf("Skipping file %s as it has no download_url", content.Name)
			}
		}

		// Sort posts by PublishDate in descending order
		sort.Slice(posts, func(i, j int) bool {
			return posts[i].PublishDate.After(posts[j].PublishDate)
		})

		if len(posts) == 0 && firstError != nil {
			return postsLoadedMsg{posts: nil, err: fmt.Errorf("failed to load any posts, first error: %w", firstError)}
		}
		// If there were non-critical errors for some files but others loaded, we still return the loaded posts.
		// The individual errors are logged.
		return postsLoadedMsg{posts: posts, err: nil}
	}
}

func (m model) Init() tea.Cmd {
	// We need to send a WindowSizeMsg to initialize the viewport correctly after the UI is up.
	// However, tea.EnterAltScreen and initial tick are also important.
	// A common pattern is to handle initial sizing in the first WindowSizeMsg.
	return tea.Batch(tick(), tea.EnterAltScreen)
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

		if !m.ready { // First WindowSizeMsg, set up viewport
			// For listScreen, we need full height minus space for footer
			footerHeight := 2 // Space for footer help text
			m.viewport = viewport.New(msg.Width, msg.Height-footerHeight)
			m.ready = true
		} else {
			// For listScreen, we need full height minus space for footer
			footerHeight := 2 // Space for footer help text
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - footerHeight
		}

		m.postList.SetWidth(msg.Width)
		m.postList.SetHeight(msg.Height) // List takes full height when active

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
				m.postList.SetItems([]list.Item{}) 
				cmds = append(cmds, fetchPostsCmd())
			}
		case listScreen:
			switch msg.String() {
			case "q", "esc":
				return m, tea.Quit
			case "b", "backspace":
				m.currentScreen = splashScreen
				m.showFlashMessage = true
				m.postsError = nil
				cmds = append(cmds, tick())
			case "up", "k":
				m.viewport.ScrollUp(1)
			case "down", "j":
				m.viewport.ScrollDown(1)
			case "pageup":
				m.viewport.ScrollUp(m.viewport.Height)
			case "pagedown":
				m.viewport.ScrollDown(m.viewport.Height)
			case "home":
				m.viewport.GotoTop()
			case "end":
				m.viewport.GotoBottom()
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
			m.postList.SetItems([]list.Item{}) 
		} else {
			items := make([]list.Item, len(msg.posts))
			for i, p := range msg.posts {
				items[i] = p 
			}
			m.postList.SetItems(items)
			m.postsError = nil
			
			// Set viewport content with the latest post
			if len(msg.posts) > 0 {
				latestPost := msg.posts[0]
				postContent := transformLinksToFootnotes(stripTags(latestPost.Content))
				glowRenderer, err := glamour.NewTermRenderer(
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(m.viewport.Width-2),
				)
				if err != nil {
					log.Printf("Error creating glamour renderer: %v", err)
					m.viewport.SetContent("Error initializing renderer.")
				} else {
					formattedContent, err := glowRenderer.Render(postContent)
					if err != nil {
						log.Printf("Error rendering markdown: %v", err)
						m.viewport.SetContent("Error rendering content.")
					} else {
						m.viewport.SetContent(formattedContent)
					}
				}
			}
		}
	}
	return m, tea.Batch(cmds...)
}

// Helper views for header/footer of postDetailScreen
func (m model) headerView() string {
	if m.selectedPost == nil {
		return ""
	}
	postTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Padding(0,1)
	return postTitleStyle.Render(m.selectedPost.PostTitle)
}

func (m model) footerView() string {
	return lipgloss.NewStyle().Padding(0,1).Render("[↑/k up, ↓/j down, q/esc/b back]")
}

func (m model) View() string {
	if !m.ready { // Don't render until viewport is initialized
		return "Initializing..."
	}

	adaptiveBackground := lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	adaptiveForeground := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}

	baseStyle := lipgloss.NewStyle().
		Background(adaptiveBackground).
		Foreground(adaptiveForeground)

	switch m.currentScreen {
	case splashScreen:
		splashContainerStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center)
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
			return loadingStyle.Render("Loading latest post...")
		}
		if m.postsError != nil {
			errorStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center)
			content := fmt.Sprintf("Error loading post: %v\n\n(Press 'q' to quit)", m.postsError)
			return errorStyle.Render(content)
		}
		if len(m.postList.Items()) > 0 {
			// Don't reset viewport content here - it was set when posts were loaded
			return lipgloss.JoinVertical(lipgloss.Left,
				m.viewport.View(),
				lipgloss.NewStyle().Padding(0, 1).Render("[↑/k up, ↓/j down, q/esc quit]"),
			)
		}
		return baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center).Render("No posts available.")


	default:
		unknownScreenStyle := baseStyle.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center)
		return unknownScreenStyle.Render("Unknown screen")
	}
}

// transformLinksToFootnotes takes a markdown string and converts inline links to footnotes.
// It returns the modified markdown and a list of URLs for the footnotes.
func transformLinksToFootnotes(markdownContent string) string {
	// Regex for [text](url) using a raw string literal for clarity and correctness.
	// Group 1: text
	// Group 2: url
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`) // Use raw string literal

	var footnotes []string
	footnoteIndex := 1

	transformedContent := re.ReplaceAllStringFunc(markdownContent, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match 
		}
		linkText := submatches[1]
		url := submatches[2]

		// Basic check to avoid re-processing if it looks like a footnote marker already
		// e.g., if linkText is "[123]"
		if strings.HasPrefix(linkText, "[") && strings.HasSuffix(linkText, "]") {
			if _, err := strconv.Atoi(linkText[1 : len(linkText)-1]); err == nil {
				return match // It's already a footnote reference like "[1]", skip.
			}
		}
		
		// Avoid re-processing if the URL part is already a footnote definition (common in some markdown outputs)
		if strings.HasPrefix(url, "#fn:") || strings.HasPrefix(url, "#fnref:") {
		    return match
		}


		footnotes = append(footnotes, url)
		newLink := fmt.Sprintf("%s [%d]", linkText, footnoteIndex)
		footnoteIndex++
		return newLink
	})

	if len(footnotes) > 0 {
		var footnotesSection strings.Builder
		footnotesSection.WriteString("\n\n---\n**Footnotes:**\n")
		for i, url := range footnotes {
			footnotesSection.WriteString(fmt.Sprintf("[%d]: %s\n", i+1, url))
		}
		transformedContent += footnotesSection.String()
	}

	return transformedContent
}

// Update the stripTags function to remove specific import statements.
func stripTags(content string) string {
	re := regexp.MustCompile(`<[^>]+>|{[^}]+}|import CallToAction from '~\/components\/widgets\/CallToAction\.astro';`)
	return re.ReplaceAllString(content, "")
}

func main() {
	// If running as an SSH app, start the SSH server
	if len(os.Args) > 1 && os.Args[1] == "ssh" {
		_, err := keygen.New("ssh_host_ed25519", keygen.WithKeyType(keygen.Ed25519))
		if err != nil {
			log.Fatalf("could not generate SSH key: %v", err)
		}
		pemBytes, err := os.ReadFile("ssh_host_ed25519")
		if err != nil {
			log.Fatalf("could not read SSH key PEM file: %v", err)
		}
		server, err := wish.NewServer(
			wish.WithAddress(":23234"), // You can change the port as needed
			wish.WithHostKeyPEM(pemBytes),
			wish.WithMiddleware(
				bubbletea.Middleware(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
					return initialModel(), nil
				}),
			),
		)
		if err != nil {
			log.Fatalf("could not start SSH server: %v", err)
		}
		log.Printf("SSH TUI server started on port 23234. Connect with: ssh -p 23234 <user>@<host>")
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("SSH server error: %v", err)
		}
		return
	}

	// Local TUI mode (default)
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatalf("could not open log file: %v", err)
	}
	defer f.Close()

	p := tea.NewProgram(initialModel())
	if _, errP := p.Run(); errP != nil {
		log.Fatalf("Error running program: %v", errP)
	}
}
