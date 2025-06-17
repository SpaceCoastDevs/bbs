# Space Coast Devs BBS

This is a Terminal User Interface (TUI) application built with Go and the Charm Bracelet and  Bubbletea ecosystem

## Features

*   **Splash Screen**: Displays an initial welcome message.
*   **Dynamic Post Fetching**: Retrieves a list of MDX files from the `SpaceCoastDevs/space-coast.dev` GitHub repository (`src/content/post` directory).
*   **Frontmatter Parsing**: Parses YAML frontmatter from each MDX file to extract metadata (title, excerpt, date, category, tags).
*   **Scrollable & Filterable List**: Uses `bubbles/list` to display posts. Users can scroll through posts and filter them by typing.
*   **Markdown Detail View**:
    *   When a post is selected, its full MDX content is fetched.
    *   Content is rendered to the terminal using `glamour`, providing basic Markdown styling.
    *   The rendered content is displayed in a scrollable view using `bubbles/viewport`.
*   **Footnote Link Conversion**: Inline Markdown links (`[text](url)`) are automatically converted to footnote style (`text [1]`) with a corresponding list of URLs at the bottom of the post. This improves readability and usability of links in the terminal.
*   **Adaptive Styling**: Uses `lipgloss` for styling, with adaptive colors for light and dark terminal themes.

## Dependencies

This project uses Go modules. Key dependencies include:

*   [github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea): The Go framework for building TUI applications.
*   [github.com/charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss): For terminal styling.
*   [github.com/charmbracelet/bubbles/list](https://github.com/charmbracelet/bubbles/list): For the scrollable/filterable list component.
*   [github.com/charmbracelet/bubbles/viewport](https://github.com/charmbracelet/bubbles/viewport): For the scrollable content view.
*   [github.com/charmbracelet/glamour](https://github.com/charmbracelet/glamour): For rendering Markdown in the terminal.
*   [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3): For parsing YAML frontmatter.

## Getting Started

### Prerequisites

*   Go (version 1.18 or later recommended).
*   A terminal that supports ANSI escape codes (most modern terminals do).

### Installation & Building

1.  **Clone the repository (if applicable) or ensure you have the source code.**
    If this project were in a git repository, you would clone it:
    ```bash
    # git clone <repository-url>
    # cd <repository-directory>
    ```
    For the current setup, ensure you are in the project directory (`/home/gilcreque/projects/ssh-space-coast.dev`).

2.  **Fetch dependencies:**
    Go modules should handle this automatically when building. If you need to explicitly fetch them:
    ```bash
    go mod tidy
    ```

3.  **Generate SSH Key**
    You need to manually generate a key for the SSH server:
    ```bash
    ssh-keygen -l -f ssh_host_ed25519
    ```

4.  **Build the application:**
    ```bash
    go build -o bbs main.go
    ```
    This will create an executable named `bbs` (or you can choose another name). If you run `go build`, the executable will be named after the project directory (e.g., `ssh-space-coast.dev`).

### Running

Execute the compiled binary:
```bash
./bbs
```
(Replace `bbs` with the actual name of your executable if you chose a different one).

Execute the compiled binary in SSH mode:
```bash
./bbs ssh
```
(Replace `bbs` with the actual name of your executable if you chose a different one).

### Controls

*   **Splash Screen**:
    *   `Enter`: Continue to the post list.
    *   `q`, `esc`, `ctrl+c`: Quit the application.
*   **Post List Screen**:
    *   `↑/k`, `↓/j`: Scroll through posts.
    *   `/`: Enter filter mode. Type to filter, `Enter` to confirm, `Esc` to clear.
    *   `Enter`: View details of the selected post.
    *   `b`, `backspace`: Go back to the splash screen.
    *   `q`, `esc`: Quit the application.
*   **Post Detail Screen**:
    *   `↑/k`, `↓/j`, `pgup`, `pgdn`, `home`, `end`: Scroll through the post content.
    *   Mouse wheel can also be used for scrolling.
    *   `b`, `backspace`, `q`, `esc`: Go back to the post list.

## Logging

The application logs debug information to `debug.log` in the same directory where it's run. This can be helpful for troubleshooting.
