// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/model"
)

var IntegrityCmd = &cobra.Command{
	Use:   "integrity",
	Short: "Check database data integrity",
	RunE:  integrityCmdF,
}

func init() {
	IntegrityCmd.Flags().Bool("confirm", false, "Confirm you really want to run a complete integrity check that may temporarily harm system performance")
	IntegrityCmd.Flags().BoolP("verbose", "v", false, "Show detailed information on integrity check results")
	RootCmd.AddCommand(IntegrityCmd)
}

func printRelationalIntegrityCheckResult(data model.RelationalIntegrityCheckData, verbose bool) {
	fmt.Printf("Found %d records in relation %s orphans of relation %s\n",
		len(data.Records), data.ChildName, data.ParentName)
	if !verbose {
		return
	}
	for _, record := range data.Records {
		var parentID string

		if record.ParentID == nil {
			parentID = "NULL"
		} else if *record.ParentID == "" {
			parentID = "empty"
		} else {
			parentID = *record.ParentID
		}

		if record.ChildID != nil {
			if parentID == "NULL" || parentID == "empty" {
				fmt.Printf("  Child %s (%s.%s) has %s ParentIdAttr (%s.%s)\n", *record.ChildID, data.ChildName, data.ChildIDAttr, parentID, data.ChildName, data.ParentIDAttr)
			} else {
				fmt.Printf("  Child %s (%s.%s) is missing Parent %s (%s.%s)\n", *record.ChildID, data.ChildName, data.ChildIDAttr, parentID, data.ChildName, data.ParentIDAttr)
			}
		} else {
			if parentID == "NULL" || parentID == "empty" {
				fmt.Printf("  Child has %s ParentIdAttr (%s.%s)\n", parentID, data.ChildName, data.ParentIDAttr)
			} else {
				fmt.Printf("  Child is missing Parent %s (%s.%s)\n", parentID, data.ChildName, data.ParentIDAttr)
			}
		}
	}
}

func printIntegrityCheckResult(result model.IntegrityCheckResult, verbose bool) {
	switch data := result.Data.(type) {
	case model.RelationalIntegrityCheckData:
		printRelationalIntegrityCheckResult(data, verbose)
	}
}

func integrityCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	confirmFlag, _ := command.Flags().GetBool("confirm")
	if !confirmFlag {
		var confirm string
		fmt.Fprintf(os.Stdout, "This check may harm performance on live systems. Are you sure you want to proceed? (y/N): ")
		fmt.Scanln(&confirm)
		if !strings.EqualFold(confirm, "y") && !strings.EqualFold(confirm, "yes") {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			return nil
		}
	}

	verboseFlag, _ := command.Flags().GetBool("verbose")
	results := a.Srv().Store.CheckIntegrity()
	for result := range results {
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", result.Err.Error())
			break
		}
		printIntegrityCheckResult(result, verboseFlag)
	}

	return nil
}
