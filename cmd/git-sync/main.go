package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/basmulder03/git-projects-sync/internal/auth"
	"github.com/basmulder03/git-projects-sync/internal/config"
	daemonpkg "github.com/basmulder03/git-projects-sync/internal/daemon"
	gitpkg "github.com/basmulder03/git-projects-sync/internal/git"
	installpkg "github.com/basmulder03/git-projects-sync/internal/install"
	"github.com/basmulder03/git-projects-sync/internal/keychain"
	"github.com/basmulder03/git-projects-sync/internal/logging"
	"github.com/basmulder03/git-projects-sync/internal/provider"
	"github.com/basmulder03/git-projects-sync/internal/registry"
	syncpkg "github.com/basmulder03/git-projects-sync/internal/sync"
)

// version is injected by goreleaser via -ldflags "-X main.version=..."
var version = "dev"

var (
	cfgPath string
	cfg     *config.Config
)

func main() {
	root := buildRoot()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "git-sync",
		Short: "Sync Git repositories from GitHub and Azure DevOps",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Skip config loading for commands that bootstrap the installation.
			switch cmd.Name() {
			case "init", "install", "uninstall", "version":
				return nil
			}
			if cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}
			loaded, err := config.Load(cfgPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config not found at %s — run 'git-sync init' first", cfgPath)
				}
				return fmt.Errorf("load config: %w", err)
			}
			cfg = loaded
			return logging.Init(cfg.General.LogLevel, config.ExpandPath(cfg.General.LogFile))
		},
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default ~/.git-sync/config.toml)")

	root.AddCommand(buildVersion())
	root.AddCommand(buildInstall())
	root.AddCommand(buildUninstall())
	root.AddCommand(buildInit())
	root.AddCommand(buildAccount())
	root.AddCommand(buildDiscover())
	root.AddCommand(buildRepo())
	root.AddCommand(buildSync())
	root.AddCommand(buildStatus())
	root.AddCommand(buildDaemon())
	root.AddCommand(buildLogs())
	root.AddCommand(buildSSH())

	return root
}

// --- version ---

func buildVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run:   func(_ *cobra.Command, _ []string) { fmt.Println(version) },
	}
}

// --- install / uninstall ---

func buildInstall() *cobra.Command {
	var (
		system      bool
		noAutostart bool
		noWizard    bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install git-sync and register as a login daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := installpkg.Install(installpkg.Options{
				System:      system,
				NoAutostart: noAutostart,
			}); err != nil {
				return err
			}

			// Ensure config exists.
			configPath := cfgPath
			if configPath == "" {
				configPath = config.DefaultConfigPath()
			}
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				if err := config.Save(config.DefaultConfig(), configPath); err != nil {
					return fmt.Errorf("create config: %w", err)
				}
				fmt.Printf("Config created at %s\n", configPath)
			}

			if noWizard {
				fmt.Println("\nDone. Run 'git-sync account add' to configure your first account.")
				return nil
			}

			loaded, err := config.Load(configPath)
			if err != nil {
				return err
			}
			cfg = loaded

			if len(cfg.Accounts) == 0 {
				fmt.Println("\nNo accounts configured — let's set up your first account.")
				if err := runAccountWizard(cfg, configPath); err != nil {
					fmt.Fprintf(os.Stderr, "Account setup: %v\n", err)
					fmt.Println("Run 'git-sync account add' to configure an account later.")
				}
			} else {
				fmt.Printf("\nDone. %d account(s) configured.\n", len(cfg.Accounts))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&system, "system", false, "install system-wide (requires admin/root)")
	cmd.Flags().BoolVar(&noAutostart, "no-autostart", false, "skip autostart registration")
	cmd.Flags().BoolVar(&noWizard, "no-wizard", false, "skip first-account setup wizard")
	return cmd
}

func buildUninstall() *cobra.Command {
	var system bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove autostart registration",
		RunE: func(_ *cobra.Command, _ []string) error {
			return installpkg.Uninstall(installpkg.Options{System: system})
		},
	}
	cmd.Flags().BoolVar(&system, "system", false, "remove system-wide registration (requires admin/root)")
	return cmd
}

// runAccountWizard interactively collects account details and saves to config.
func runAccountWizard(c *config.Config, configPath string) error {
	scanner := bufio.NewScanner(os.Stdin)

	prompt := func(question string) string {
		fmt.Print(question)
		scanner.Scan()
		return strings.TrimSpace(scanner.Text())
	}
	promptDefault := func(question, def string) string {
		fmt.Printf("%s [%s]: ", question, def)
		scanner.Scan()
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			return def
		}
		return v
	}
	confirmYes := func(question string) bool {
		fmt.Printf("%s [Y/n]: ", question)
		scanner.Scan()
		v := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return v == "" || v == "y" || v == "yes"
	}

	// Provider
	prov := prompt("Provider (github / azure_devops): ")
	switch prov {
	case "github", "azure_devops":
	default:
		return fmt.Errorf("unknown provider %q — must be github or azure_devops", prov)
	}

	// Account ID
	id := prompt("Account ID (e.g. github-personal): ")
	if id == "" {
		return fmt.Errorf("account ID cannot be empty")
	}
	for _, a := range c.Accounts {
		if a.ID == id {
			return fmt.Errorf("account %q already exists", id)
		}
	}

	var username, org string
	if prov == "github" {
		username = prompt("GitHub username: ")
	} else {
		org = prompt("Azure DevOps organization: ")
	}

	defaultKey := fmt.Sprintf("~/.ssh/git-sync-%s", id)
	sshKey := promptDefault("SSH key path", defaultKey)

	cloneMethod := promptDefault("Clone method (ssh / https)", "ssh")
	if cloneMethod != "ssh" && cloneMethod != "https" {
		cloneMethod = "ssh"
	}

	account := config.AccountConfig{
		ID:           id,
		Provider:     prov,
		Username:     username,
		Organization: org,
		SSHKeyPath:   sshKey,
		CloneMethod:  cloneMethod,
	}
	c.Accounts = append(c.Accounts, account)

	if cloneMethod == "ssh" {
		if confirmYes(fmt.Sprintf("Generate SSH key at %s", sshKey)) {
			if err := gitpkg.GenerateSSHKey(sshKey, "git-sync:"+id, false); err != nil {
				fmt.Fprintf(os.Stderr, "  Key generation: %v\n", err)
			} else {
				if err := gitpkg.EnsureSSHEntry(account); err != nil {
					fmt.Fprintf(os.Stderr, "  SSH config: %v\n", err)
				} else {
					fmt.Printf("  SSH config entry: %s\n", gitpkg.SSHHostAlias(account))
				}
				pubKey, err := gitpkg.ReadPublicKey(sshKey)
				if err == nil {
					fmt.Printf("\nPublic key (add this to your provider):\n\n%s\n\n", pubKey)
				}
				if url := gitpkg.ProviderKeyURL(account); url != "" {
					fmt.Printf("Add at: %s\n\n", url)
				}
			}
		}
	}

	switch prov {
	case "github":
		fmt.Println("\nPAT required scopes: repo  (or public_repo for public repos only)")
		fmt.Println("Create at: https://github.com/settings/tokens")
	case "azure_devops":
		fmt.Println("\nPAT required scopes: Code › Read  (scoped to your organization)")
		fmt.Println("Create at: https://dev.azure.com/{your-org}/_usersSettings/tokens")
	}
	if confirmYes(fmt.Sprintf("Store PAT for %q in keychain now", id)) {
		if err := promptAndStorePAT(id); err != nil {
			fmt.Fprintf(os.Stderr, "  PAT: %v\n", err)
		}
	}

	if err := config.Save(c, configPath); err != nil {
		return err
	}
	fmt.Printf("\nAccount %q configured!\n", id)
	fmt.Printf("Run 'git-sync discover %s --add' to discover and track repositories.\n", id)
	return nil
}

// --- init ---

func buildInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create default config file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := cfgPath
			if path == "" {
				path = config.DefaultConfigPath()
			}
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("Config already exists at %s\n", path)
				return nil
			}
			if err := config.Save(config.DefaultConfig(), path); err != nil {
				return err
			}
			fmt.Printf("Config created at %s\n", path)
			return nil
		},
	}
}

// --- account ---

func buildAccount() *cobra.Command {
	acc := &cobra.Command{
		Use:   "account",
		Short: "Manage provider accounts",
	}
	acc.AddCommand(buildAccountAdd())
	acc.AddCommand(buildAccountList())
	acc.AddCommand(buildAccountRemove())
	acc.AddCommand(buildAccountSetPAT())
	return acc
}

func buildAccountAdd() *cobra.Command {
	var (
		provider    string
		id          string
		username    string
		org         string
		sshKey      string
		cloneMethod string
		setPAT      bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a provider account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			account := config.AccountConfig{
				ID:           id,
				Provider:     provider,
				Username:     username,
				Organization: org,
				SSHKeyPath:   sshKey,
				CloneMethod:  cloneMethod,
			}
			if cloneMethod == "" {
				account.CloneMethod = "ssh"
			}

			// Check duplicate ID.
			for _, a := range cfg.Accounts {
				if a.ID == id {
					return fmt.Errorf("account %q already exists", id)
				}
			}
			cfg.Accounts = append(cfg.Accounts, account)

			if sshKey != "" {
				if err := gitpkg.EnsureSSHEntry(account); err != nil {
					fmt.Fprintf(os.Stderr, "warning: SSH config update failed: %v\n", err)
				}
			}

			if setPAT {
				if err := promptAndStorePAT(id); err != nil {
					return err
				}
			}

			if err := config.Save(cfg, cfgPath); err != nil {
				return err
			}
			fmt.Printf("Account %q added\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "github or azure_devops (required)")
	cmd.Flags().StringVar(&id, "id", "", "unique account ID (required)")
	cmd.Flags().StringVar(&username, "username", "", "GitHub username or AzDo username")
	cmd.Flags().StringVar(&org, "org", "", "Azure DevOps organisation")
	cmd.Flags().StringVar(&sshKey, "ssh-key", "", "path to SSH private key")
	cmd.Flags().StringVar(&cloneMethod, "clone-method", "ssh", "ssh or https")
	cmd.Flags().BoolVar(&setPAT, "set-pat", false, "prompt for PAT and store in keychain")
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func buildAccountList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts",
		RunE: func(_ *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPROVIDER\tUSERNAME\tCLONE")
			for _, a := range cfg.Accounts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Provider, a.Username, a.CloneMethod)
			}
			return w.Flush()
		},
	}
}

func buildAccountRemove() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a provider account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			found := false
			accounts := cfg.Accounts[:0]
			for _, a := range cfg.Accounts {
				if a.ID == id {
					found = true
					_ = gitpkg.RemoveSSHEntry(id)
					_ = keychain.DeletePAT(id)
					continue
				}
				accounts = append(accounts, a)
			}
			if !found {
				return fmt.Errorf("account %q not found", id)
			}
			cfg.Accounts = accounts
			if err := config.Save(cfg, cfgPath); err != nil {
				return err
			}
			fmt.Printf("Account %q removed\n", id)
			return nil
		},
	}
}

func buildAccountSetPAT() *cobra.Command {
	return &cobra.Command{
		Use:   "set-pat <id>",
		Short: "Store a PAT for an account in the OS keychain",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return promptAndStorePAT(args[0])
		},
	}
}

func promptAndStorePAT(accountID string) error {
	fmt.Printf("Enter PAT for account %q: ", accountID)
	var token string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read PAT: %w", err)
		}
		token = string(raw)
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		token = scanner.Text()
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("PAT cannot be empty")
	}
	if err := keychain.SetPAT(accountID, token); err != nil {
		return err
	}
	fmt.Printf("PAT stored for account %q\n", accountID)
	return nil
}

// --- discover ---

func buildDiscover() *cobra.Command {
	var (
		add             bool
		includeArchived bool
	)
	cmd := &cobra.Command{
		Use:   "discover <account-id>",
		Short: "Discover repositories for an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			accountID := args[0]
			account, err := findAccount(accountID)
			if err != nil {
				return err
			}

			a := auth.NewPATAuth(accountID)
			prov, err := registry.NewProvider(*account, a)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			repos, err := prov.ListRepositories(ctx, provider.ListOptions{
				IncludeArchived: includeArchived,
				PageSize:        100,
			})
			if err != nil {
				return err
			}

			fmt.Printf("%s (%s) — %s\n", titleCase(account.Provider), account.ID, account.Username)
			for i, r := range repos {
				prefix := "├──"
				if i == len(repos)-1 {
					prefix = "└──"
				}
				fmt.Printf("%s %s  (SSH: %s)\n", prefix, r.FullName, r.SSHURL)
			}

			if add {
				existing := map[string]bool{}
				for _, r := range cfg.Repositories {
					existing[r.FullName] = true
				}
				added := 0
				for _, r := range repos {
					if existing[r.FullName] {
						continue
					}
					localPath := deriveLocalPath(cfg.General.WorkspaceRoot, *account, r)
					cfg.Repositories = append(cfg.Repositories, config.RepoConfig{
						AccountID:     accountID,
						FullName:      r.FullName,
						LocalPath:     localPath,
						AutoSync:      true,
						DefaultBranch: r.DefaultBranch,
					})
					added++
				}
				if err := config.Save(cfg, cfgPath); err != nil {
					return err
				}
				fmt.Printf("\nAdded %d repositories to config\n", added)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&add, "add", false, "add discovered repos to config")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "include archived repos")
	return cmd
}

func deriveLocalPath(workspaceRoot string, account config.AccountConfig, r *provider.Repository) string {
	base := config.ExpandPath(workspaceRoot)
	parts := strings.Split(r.FullName, "/")
	segments := append([]string{base, account.Provider}, parts...)
	return filepath.Join(segments...)
}

// --- repo ---

func buildRepo() *cobra.Command {
	r := &cobra.Command{
		Use:   "repo",
		Short: "Manage tracked repositories",
	}
	r.AddCommand(buildRepoAdd())
	r.AddCommand(buildRepoList())
	r.AddCommand(buildRepoRemove())
	return r
}

func buildRepoAdd() *cobra.Command {
	var accountID, name string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a repository to tracking",
		RunE: func(_ *cobra.Command, _ []string) error {
			account, err := findAccount(accountID)
			if err != nil {
				return err
			}
			localPath := deriveLocalPath(cfg.General.WorkspaceRoot, *account, &provider.Repository{FullName: name})
			cfg.Repositories = append(cfg.Repositories, config.RepoConfig{
				AccountID: accountID,
				FullName:  name,
				LocalPath: localPath,
				AutoSync:  true,
			})
			if err := config.Save(cfg, cfgPath); err != nil {
				return err
			}
			fmt.Printf("Repository %q added\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "account ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "repo full name e.g. owner/repo (required)")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func buildRepoList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracked repositories",
		RunE: func(_ *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ACCOUNT\tFULL NAME\tLOCAL PATH\tAUTO SYNC")
			for _, r := range cfg.Repositories {
				fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", r.AccountID, r.FullName, r.LocalPath, r.AutoSync)
			}
			return w.Flush()
		},
	}
}

func buildRepoRemove() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <full-name>",
		Short: "Remove a repository from tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			found := false
			repos := cfg.Repositories[:0]
			for _, r := range cfg.Repositories {
				if r.FullName == name {
					found = true
					continue
				}
				repos = append(repos, r)
			}
			if !found {
				return fmt.Errorf("repository %q not found in config", name)
			}
			cfg.Repositories = repos
			if err := config.Save(cfg, cfgPath); err != nil {
				return err
			}
			fmt.Printf("Repository %q removed\n", name)
			return nil
		},
	}
}

// --- sync ---

func buildSync() *cobra.Command {
	var (
		dryRun  bool
		repoArg string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run a one-shot sync",
		RunE: func(_ *cobra.Command, _ []string) error {
			if dryRun {
				fmt.Println("[dry-run] would sync the following repositories:")
				for _, r := range cfg.Repositories {
					if !r.AutoSync {
						continue
					}
					if repoArg != "" && r.LocalPath != repoArg && r.FullName != repoArg {
						continue
					}
					fmt.Printf("  %s  (%s)\n", r.FullName, config.ExpandPath(r.LocalPath))
				}
				return nil
			}

			syncer := syncpkg.New(cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			if repoArg != "" {
				for _, r := range cfg.Repositories {
					if r.LocalPath == repoArg || r.FullName == repoArg {
						return syncer.SyncRepo(ctx, r)
					}
				}
				return fmt.Errorf("repository %q not found in config", repoArg)
			}

			return syncer.SyncAll(ctx)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen without doing it")
	cmd.Flags().StringVar(&repoArg, "repo", "", "sync only this repo (path or full-name)")
	return cmd
}

// --- status ---

func buildStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of all tracked repositories",
		RunE: func(_ *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for _, r := range cfg.Repositories {
				localPath := config.ExpandPath(r.LocalPath)
				if !gitpkg.IsRepo(localPath) {
					fmt.Fprintf(w, "-\t%s\t\tnot cloned\n", shorten(localPath))
					continue
				}
				status, err := gitpkg.GetStatus(localPath)
				if err != nil {
					fmt.Fprintf(w, "?\t%s\t\terror: %v\n", shorten(localPath), err)
					continue
				}

				icon := "✓"
				detail := "clean"
				if status.IsDirty {
					icon = "✗"
					detail = "dirty"
				} else if status.BehindCount > 0 {
					icon = "↓"
					detail = fmt.Sprintf("behind %d", status.BehindCount)
				}

				fmt.Fprintf(w, "%s\t%s\t[%s]\t%s\n", icon, shorten(localPath), status.CurrentBranch, detail)
			}
			return w.Flush()
		},
	}
}

func shorten(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return path
	}
	return "~" + string(filepath.Separator) + rel
}

// --- daemon ---

func buildDaemon() *cobra.Command {
	d := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background sync daemon",
	}

	d.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start the background daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			dm := daemonpkg.New(cfg)
			if err := dm.Start(); err != nil {
				return err
			}
			fmt.Println("Daemon started")
			// Block so the process keeps running.
			select {}
		},
	})

	d.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the background daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			dm := daemonpkg.New(cfg)
			if !dm.IsRunning() {
				fmt.Println("Daemon is not running")
				return nil
			}
			return dm.Stop()
		},
	})

	d.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(_ *cobra.Command, _ []string) error {
			dm := daemonpkg.New(cfg)
			if dm.IsRunning() {
				fmt.Println("Daemon is running")
			} else {
				fmt.Println("Daemon is not running")
			}
			return nil
		},
	})

	return d
}

// --- logs ---

func buildLogs() *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show log file contents",
		RunE: func(_ *cobra.Command, _ []string) error {
			logFile := config.ExpandPath(cfg.General.LogFile)
			f, err := os.Open(logFile)
			if err != nil {
				return fmt.Errorf("open log file: %w", err)
			}
			defer f.Close()

			var all []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				all = append(all, scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return err
			}

			start := 0
			if lines > 0 && len(all) > lines {
				start = len(all) - lines
			}
			for _, line := range all[start:] {
				fmt.Println(line)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&lines, "lines", 50, "number of lines to show")
	return cmd
}

// --- ssh ---

func buildSSH() *cobra.Command {
	ssh := &cobra.Command{
		Use:   "ssh",
		Short: "Manage SSH keys for provider accounts",
	}
	ssh.AddCommand(buildSSHGenerate())
	ssh.AddCommand(buildSSHShowPubkey())
	ssh.AddCommand(buildSSHTest())
	ssh.AddCommand(buildSSHStatus())
	return ssh
}

func buildSSHGenerate() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "generate <account-id>",
		Short: "Generate an ed25519 SSH key pair for an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			account, err := findAccount(args[0])
			if err != nil {
				return err
			}
			if account.SSHKeyPath == "" {
				return fmt.Errorf("account %q has no ssh_key_path configured", account.ID)
			}

			comment := fmt.Sprintf("git-sync:%s", account.ID)
			if err := gitpkg.GenerateSSHKey(account.SSHKeyPath, comment, force); err != nil {
				return err
			}
			fmt.Printf("Key generated: %s\n", config.ExpandPath(account.SSHKeyPath))

			// Update ~/.ssh/config entry.
			if err := gitpkg.EnsureSSHEntry(*account); err != nil {
				fmt.Fprintf(os.Stderr, "warning: SSH config update failed: %v\n", err)
			} else {
				fmt.Printf("SSH config updated for host alias: %s\n", gitpkg.SSHHostAlias(*account))
			}

			// Show public key and provider URL.
			pubKey, err := gitpkg.ReadPublicKey(account.SSHKeyPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not read public key: %v\n", err)
			} else {
				fmt.Printf("\nPublic key (add this to your provider):\n\n%s\n", pubKey)
			}

			if url := gitpkg.ProviderKeyURL(*account); url != "" {
				fmt.Printf("\nAdd at: %s\n", url)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing key")
	return cmd
}

func buildSSHShowPubkey() *cobra.Command {
	return &cobra.Command{
		Use:   "show-pubkey <account-id>",
		Short: "Print the public key and provider URL for an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			account, err := findAccount(args[0])
			if err != nil {
				return err
			}
			if account.SSHKeyPath == "" {
				return fmt.Errorf("account %q has no ssh_key_path configured", account.ID)
			}

			pubKey, err := gitpkg.ReadPublicKey(account.SSHKeyPath)
			if err != nil {
				return err
			}
			fmt.Println(pubKey)

			if url := gitpkg.ProviderKeyURL(*account); url != "" {
				fmt.Printf("\nAdd at: %s\n", url)
			}
			return nil
		},
	}
}

func buildSSHTest() *cobra.Command {
	return &cobra.Command{
		Use:   "test <account-id>",
		Short: "Test the SSH connection for an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			account, err := findAccount(args[0])
			if err != nil {
				return err
			}

			alias := gitpkg.SSHHostAlias(*account)
			fmt.Printf("Testing SSH connection to %s ...\n", alias)

			out, success, err := gitpkg.TestSSHConnection(*account)
			if err != nil {
				return fmt.Errorf("ssh test: %w", err)
			}
			if out != "" {
				fmt.Printf("Server response: %s\n", out)
			}
			if success {
				fmt.Println("Authentication successful")
			} else {
				fmt.Fprintln(os.Stderr, "Authentication failed — check key is added to provider")
				os.Exit(1)
			}
			return nil
		},
	}
}

func buildSSHStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show SSH key status for all accounts",
		RunE: func(_ *cobra.Command, _ []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ACCOUNT\tPROVIDER\tKEY PATH\tKEY EXISTS\tSSH CONFIG")
			for _, account := range cfg.Accounts {
				keyExists := "no"
				if account.SSHKeyPath != "" && gitpkg.SSHKeyExists(account.SSHKeyPath) {
					keyExists = "yes"
				}
				sshConfig := "no"
				if ok, _ := gitpkg.SSHConfigEntryExists(account.ID); ok {
					sshConfig = "yes"
				}
				keyPath := account.SSHKeyPath
				if keyPath == "" {
					keyPath = "(not set)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					account.ID, account.Provider, keyPath, keyExists, sshConfig)
			}
			return w.Flush()
		},
	}
}

// --- helpers ---

func findAccount(id string) (*config.AccountConfig, error) {
	for i, a := range cfg.Accounts {
		if a.ID == id {
			return &cfg.Accounts[i], nil
		}
	}
	return nil, fmt.Errorf("account %q not found", id)
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
