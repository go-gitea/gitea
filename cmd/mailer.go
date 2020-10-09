package cmd

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/mailer"
	"github.com/urfave/cli"
)

func runSendMail(c *cli.Context) error {
	if err := argsSet(c, "title", "content"); err != nil {
		return err
	}

	subject := c.String("title")
	body := c.String("content")
	confirmSkiped := c.Bool("force")

	if err := initDB(); err != nil {
		return err
	}

	users, _, err := models.SearchUsers(&models.SearchUserOptions{
		Type:        models.UserTypeIndividual,
		OrderBy:     models.SearchOrderByAlphabetically,
		ListOptions: models.ListOptions{},
	})
	if err != nil {
		return errors.New("Cann't find users")
	}

	var emails []string
	for _, user := range users {
		emails = append(emails, user.Email)
	}

	if !confirmSkiped {
		fmt.Print("Proceed with sending email? [Y/n] ")
		isConfirmed, err := confirm()
		if err != nil {
			return err
		} else if !isConfirmed {
			fmt.Println("The mail was not sent")
			return nil
		}
	}

	mailer.NewContext()
	msg := mailer.NewMessage(emails, subject, body)
	err = mailer.SendSync(msg)
	if err != nil {
		return err
	}

	return nil
}
