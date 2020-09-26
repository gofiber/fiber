package cmd

import (
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2/fiber/tpl"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"text/template"
)

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new [projectName]",
	Short: "Create new fiber project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 || args[0] == "" {
			return errors.New("project name not specify")
		}

		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		return newProject(args[0], wd)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

func newProject(projectName, dir string) error {
	path := fmt.Sprintf("%s/%s", dir, projectName)
	if err := os.Mkdir(path, os.ModePerm); err != nil {
		return err
	}

	if err := os.Chdir(path); err != nil {
		return err
	}

	// create main.go
	mainFile, err := os.Create(fmt.Sprintf("%s/main.go", path))
	if err != nil {
		return err
	}
	defer func() {
		if err := mainFile.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	mainTemplate := template.Must(template.New("main").Parse(string(tpl.MainTemplate())))
	if err := mainTemplate.Execute(mainFile, nil); err != nil {
		return err
	}

	cmdInit := exec.Command("go", "mod", "init", projectName)
	if err := cmdInit.Run(); err != nil {
		return err
	}

	cmdTidy := exec.Command("go", "mod", "tidy")
	if err := cmdTidy.Run(); err != nil {
		return err
	}

	log.Printf("Created %s project\n", projectName)
	log.Println("Get started by running")
	log.Printf("cd %s && go run main.go ", projectName)

	return nil
}
