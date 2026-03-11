package export

import (
	"context"
	"regexp"
	"strings"

	"dump-todos-go/internal/graph"
)

type List struct {
	DisplayName string       `json:"displayName"`
	Tasks       []graph.Task `json:"tasks"`
}

var (
	htmlTagPattern    = regexp.MustCompile(`<[^>]+>`)
	whitespacePattern = regexp.MustCompile(`\s+`)
)

func Markdown(ctx context.Context, client *graph.Client, lists []graph.TodoList, incompleteOnly bool) (string, error) {
	exportLists := make([]List, 0, len(lists))

	for _, list := range lists {
		tasks, err := client.Tasks(ctx, list.ID)
		if err != nil {
			return "", err
		}

		exportLists = append(exportLists, List{
			DisplayName: list.DisplayName,
			Tasks:       tasks,
		})
	}

	return RenderLists(exportLists, incompleteOnly), nil
}

func RenderLists(lists []List, incompleteOnly bool) string {
	lines := make([]string, 0, len(lists)*4)

	for _, list := range lists {
		filtered := list.Tasks
		if incompleteOnly {
			filtered = make([]graph.Task, 0, len(list.Tasks))
			for _, task := range list.Tasks {
				if task.Status != "completed" {
					filtered = append(filtered, task)
				}
			}
		}

		if len(filtered) == 0 {
			continue
		}

		lines = append(lines, "# "+list.DisplayName)
		for _, task := range filtered {
			done := " "
			if task.Status == "completed" {
				done = "x"
			}

			due := ""
			if task.DueDateTime != nil && len(task.DueDateTime.DateTime) >= 10 {
				due = " (due: " + task.DueDateTime.DateTime[:10] + ")"
			}

			lines = append(lines, "- ["+done+"] "+task.Title+due)

			if note := normalizeBody(task.Body); note != "" {
				lines = append(lines, "  - Note: "+note)
			}

			for _, item := range task.ChecklistItems {
				checked := " "
				if item.IsChecked {
					checked = "x"
				}
				lines = append(lines, "  - ["+checked+"] "+item.DisplayName)
			}
		}

		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func normalizeBody(body graph.ItemBody) string {
	content := strings.TrimSpace(body.Content)
	if content == "" {
		return ""
	}

	if body.ContentType == "html" {
		content = htmlTagPattern.ReplaceAllString(content, " ")
		content = strings.ReplaceAll(content, "&nbsp;", " ")
	}

	content = whitespacePattern.ReplaceAllString(content, " ")
	return strings.TrimSpace(content)
}
