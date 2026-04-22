package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initConfigFile string

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a workspace",
	Long: `Initialize a new xr workspace. Creates the directory structure,
initializes a git repository, adds submodules for remote repos,
and creates symlinks for local repos.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolving directory: %w", err)
		}

		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			fmt.Printf("Directory %s does not exist. Create it? [y/N]: ", absDir)
			answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
			if err := os.MkdirAll(absDir, 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
		} else if dir == "." {
			fmt.Printf("Initialize workspace in current directory (%s)? [y/N]: ", absDir)
			answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		cfgPath := initConfigFile
		if cfgPath == "" {
			cfgPath = filepath.Join(absDir, "repos.yaml")
		}

		var cfg *config.Config
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			fmt.Printf("Config file not found: %s\n", cfgPath)
			fmt.Println("  1) Create repos.yaml now")
			fmt.Println("  2) Initialize without repos (creates README only)")
			fmt.Print("Choose [1/2]: ")
			choice, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			choice = strings.TrimSpace(choice)

			switch choice {
			case "1":
				created, err := runConfigWizard(cfgPath)
				if err != nil {
					return fmt.Errorf("creating config: %w", err)
				}
				cfg = created
			case "2":
				cfg = &config.Config{Workspace: "./repos"}
			default:
				fmt.Println("Aborted.")
				return nil
			}
		} else {
			loaded, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			cfg = loaded
		}

		fmt.Printf("Initializing workspace in %s\n", absDir)

		ws := workspace.New(absDir, cfg)
		if err := ws.Init(); err != nil {
			return fmt.Errorf("initializing workspace: %w", err)
		}

		fmt.Printf("\nAdd repos directory (%s) to .gitignore? [y/N]: ", cfg.Workspace)
		ignoreAnswer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		ignoreAnswer = strings.TrimSpace(strings.ToLower(ignoreAnswer))
		ignoreWorkspace := ignoreAnswer == "y" || ignoreAnswer == "yes"

		if err := ws.CreateGitignore(ignoreWorkspace); err != nil {
			return fmt.Errorf("creating .gitignore: %w", err)
		}

		fmt.Printf("\nWorkspace initialized successfully.\n")
		fmt.Printf("Run 'xr sync --submodules' to sync submodules.\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&initConfigFile, "file", "f", "", "repos.yaml config file path")
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	val, _ := reader.ReadString('\n')
	val = strings.TrimSpace(val)
	if val == "" {
		return defaultVal
	}
	return val
}

func runConfigWizard(cfgPath string) (*config.Config, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n--- repos.yaml setup ---")
	fmt.Println("(Enter 'redo' at any confirmation to re-enter, empty input accepts the default)")

	var wsDir string
	for {
		wsDir = prompt(reader, "Workspace directory", "./repos")
		fmt.Printf("  Workspace: %s  [Y/n/redo]: ", wsDir)
		ans, _ := reader.ReadString('\n')
		ans = strings.ToLower(strings.TrimSpace(ans))
		if ans == "redo" {
			continue
		}
		if ans == "n" {
			fmt.Println("Aborted.")
			return nil, fmt.Errorf("cancelled by user")
		}
		break
	}
	cfg := &config.Config{Workspace: wsDir}

	for {
		fmt.Print("\nAdd a repository? [y/N]: ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) != "y" {
			break
		}

		var repo config.Repository
		for {
			name := prompt(reader, "  Name", "")
			if name == "" {
				fmt.Println("  Name is required.")
				continue
			}
			source := prompt(reader, "  Source (URL or local path)", "")
			if source == "" {
				fmt.Println("  Source is required.")
				continue
			}
			branch := prompt(reader, "  Branch", "main")
			repoPath := prompt(reader, "  Path (relative in workspace)", name)

			fmt.Printf("  → name: %s, source: %s, branch: %s, path: %s\n", name, source, branch, repoPath)
			fmt.Print("  Confirm? [Y/n/redo]: ")
			confirm, _ := reader.ReadString('\n')
			confirm = strings.ToLower(strings.TrimSpace(confirm))

			if confirm == "redo" {
				fmt.Println("  Re-entering...")
				continue
			}
			if confirm == "n" {
				fmt.Println("  Skipped.")
				break
			}
			repo = config.Repository{
				Name:   name,
				Source: source,
				Branch: branch,
				Path:   repoPath,
			}
			cfg.Repositories = append(cfg.Repositories, repo)
			fmt.Printf("  Added: %s\n", name)
			break
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return nil, err
	}
	fmt.Printf("\nCreated %s\n", cfgPath)
	return cfg, nil
}
