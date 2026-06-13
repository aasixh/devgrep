package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	searchpkg "github.com/aasixh/devgrep/internal/search"
	"github.com/aasixh/devgrep/internal/storage"
	"github.com/aasixh/devgrep/internal/utils"
	"github.com/spf13/cobra"
)

func newSourcesCommand() *cobra.Command {
	var tree bool
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Show indexed source locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			store, err := openStore(ctx, cfg)
			if err != nil {
				return err
			}
			defer store.Close()
			locations, err := store.SourceLocations(ctx)
			if err != nil {
				return err
			}
			if tree {
				printSourcesTree(cmd.OutOrStdout(), locations)
				return nil
			}
			printSources(cmd.OutOrStdout(), locations)
			return nil
		},
	}
	cmd.Flags().BoolVar(&tree, "tree", false, "render indexed source directories as a tree")
	return cmd
}

func printSources(w io.Writer, locations []storage.SourceLocation) {
	grouped := groupSourceLocations(locations)
	order := []string{searchpkg.SourceHistory, searchpkg.SourceNote, searchpkg.SourceLog}
	labels := map[string]string{
		searchpkg.SourceHistory: "history",
		searchpkg.SourceNote:    "notes",
		searchpkg.SourceLog:     "logs",
	}
	for _, source := range order {
		paths := grouped[source]
		if len(paths) == 0 {
			continue
		}
		fmt.Fprintf(w, "[%s]\n", labels[source])
		for _, path := range paths {
			fmt.Fprintln(w, utils.RelHome(path))
		}
		fmt.Fprintln(w)
	}
}

func printSourcesTree(w io.Writer, locations []storage.SourceLocation) {
	paths := uniqueSourceDirs(locations)
	if len(paths) == 0 {
		fmt.Fprintln(w, "No indexed sources found.")
		return
	}
	root := commonRoot(paths)
	fmt.Fprintln(w, utils.RelHome(root))
	tree := map[string][]string{}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			continue
		}
		parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
		parent := ""
		for i, part := range parts {
			child := filepath.Join(parent, part)
			if !contains(tree[parent], part) {
				tree[parent] = append(tree[parent], part)
			}
			if i < len(parts)-1 {
				parent = child
			}
		}
	}
	for parent := range tree {
		sort.Strings(tree[parent])
	}
	renderTree(w, tree, "", "")
}

func renderTree(w io.Writer, tree map[string][]string, parent, prefix string) {
	children := tree[parent]
	for i, child := range children {
		last := i == len(children)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if last {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		fmt.Fprintln(w, prefix+connector+child)
		key := filepath.Join(parent, child)
		renderTree(w, tree, key, nextPrefix)
	}
}

func groupSourceLocations(locations []storage.SourceLocation) map[string][]string {
	grouped := map[string][]string{}
	seen := map[string]struct{}{}
	for _, location := range locations {
		path := displaySourcePath(location)
		key := location.Type + "\x00" + path
		if _, ok := seen[key]; ok || path == "" {
			continue
		}
		seen[key] = struct{}{}
		grouped[location.Type] = append(grouped[location.Type], path)
	}
	for source := range grouped {
		sort.Strings(grouped[source])
	}
	return grouped
}

func displaySourcePath(location storage.SourceLocation) string {
	switch location.Type {
	case searchpkg.SourceHistory:
		return location.Path
	case searchpkg.SourceLog, searchpkg.SourceNote:
		return filepath.Dir(location.Path)
	default:
		return location.Path
	}
}

func uniqueSourceDirs(locations []storage.SourceLocation) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, location := range locations {
		path := displaySourcePath(location)
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}
	sort.Strings(paths)
	return paths
}

func commonRoot(paths []string) string {
	if len(paths) == 0 {
		return "."
	}
	root := filepath.Clean(paths[0])
	for _, path := range paths[1:] {
		for {
			rel, err := filepath.Rel(root, path)
			if err == nil && !strings.HasPrefix(rel, "..") {
				break
			}
			next := filepath.Dir(root)
			if next == root {
				return root
			}
			root = next
		}
	}
	return root
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
