// Command memory is an opt-in operations tool for the typed-lane Shared Memory
// subsystem. It is NOT auto-wired to the running server; operators run it
// deliberately to seed candidates from repository Markdown (a second candidate
// source beyond session-close text).
//
// Usage:
//
//	memory scan --root docs --workspace . --operator operator
//
// Submitted candidates land as pending lane entries and still require human
// disposition before they become canonical truth (M5 提交权).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/settings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
	case "reflect":
		runReflect(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: memory <command> [flags]")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  scan     scan repo Markdown for memory candidates (opt-in seed)")
	fmt.Fprintln(os.Stderr, "  reflect  LLM abstractive reflection over approved truth (opt-in; needs SG_REFLECT_* env)")
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	root := fs.String("root", "docs", "directory to scan for .md files")
	workspace := fs.String("workspace", ".", "workspace dir; determines ConfigRoot for the lane store")
	operator := fs.String("operator", "operator", "operator scope for submitted candidates")
	fs.Parse(args)

	reg := memory.NewLaneRegistryAt(filepath.Join(settings.ConfigRoot(*workspace), "lanes.json"))
	defer reg.Close()

	scanner := memory.NewRepoScanner()
	n := scanner.RunScan(reg, *root, *operator)
	fmt.Printf("scanned %s: submitted %d new memory candidate(s) (operator=%s)\n", *root, n, *operator)
}

// runReflect synthesizes an abstractive reflection over approved truth via the
// sanctioned LLM synthesis service (不可逆决策 §4.8). It is opt-in:
// the model is built from SG_REFLECT_* env and the output is printed (it never
// auto-becomes truth). With -seed it is submitted as a PENDING candidate that
// still requires human disposition (M5 提交权).
func runReflect(args []string) {
	fs := flag.NewFlagSet("reflect", flag.ExitOnError)
	workspace := fs.String("workspace", ".", "workspace dir; determines ConfigRoot for the lane store")
	operator := fs.String("operator", "operator", "operator scope for recalled truth")
	lane := fs.String("lane", "", "optional lane filter (decision/lesson/taste/profile/entity/event)")
	focus := fs.String("focus", "", "optional reflection focus directive")
	seed := fs.Bool("seed", false, "submit the reflection as a pending candidate (human disposition required)")
	fs.Parse(args)

	ctx := context.Background()
	chat, err := capability.NewReflectModelFromEnv(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reflection unavailable:", err)
		os.Exit(1)
	}

	reg := memory.NewLaneRegistryAt(filepath.Join(settings.ConfigRoot(*workspace), "lanes.json"))
	defer reg.Close()

	entries, ok := reg.RecallEntries(100, *operator)
	if !ok {
		fmt.Println("no approved truth to reflect on")
		return
	}
	if *lane != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if string(e.Type) == *lane {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if len(entries) == 0 {
		fmt.Println("no approved truth to reflect on")
		return
	}

	refl, err := capability.NewMemoryReflect(chat).Reflect(ctx, entries, capability.ReflectOptions{Focus: *focus})
	if err != nil {
		fmt.Fprintln(os.Stderr, "reflection failed:", err)
		os.Exit(1)
	}
	fmt.Println(refl)
	if *seed {
		seedLane := memory.LaneDecision
		if *lane != "" {
			if l := reg.Lane(memory.LaneType(*lane)); l != nil {
				seedLane = memory.LaneType(*lane)
			}
		}
		ids := memory.NewDeltaProducer().SubmitCandidates(reg, []memory.DeltaCandidate{{
			Lane:    seedLane,
			Content: refl,
			Source:  "reflection:" + *operator,
		}}, *operator)
		fmt.Printf("seeded %d pending candidate(s) (awaiting human disposition)\n", len(ids))
	}
}
