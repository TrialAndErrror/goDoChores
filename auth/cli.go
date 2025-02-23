package auth

import (
	"fmt"
	"github.com/spf13/cobra"
	"goDoChores/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

var CreateUserCmd = &cobra.Command{
	Use:   "create-user",
	Short: "Create a new user",
	Run: func(cmd *cobra.Command, args []string) {
		db, dbConnectErr := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
		if dbConnectErr != nil {
			panic("failed to connect database")
		}

		var username, email, password string

		fmt.Print("Enter username: ")
		_, usernameErr := fmt.Scanln(&username)
		if usernameErr != nil {
			log.Fatal(usernameErr)
		}

		fmt.Print("Enter email: ")
		_, emailErr := fmt.Scanln(&email)
		if emailErr != nil {
			log.Fatal(emailErr)
		}

		fmt.Print("Enter password: ")
		_, passwordErr := fmt.Scanln(&password)
		if passwordErr != nil {
			log.Fatal(passwordErr)
		}

		// Create user and save to database
		var existingUser models.User
		db.Where(models.User{Username: username}).First(&models.User{})
		if existingUser.Username != "" {
			fmt.Println("User already exists")
			existingUser.Email = email
			existingUser.Username = username
			db.Save(&existingUser)

			if err := existingUser.SetPassword(password); err != nil {
				log.Fatal("Error hashing password:", err)
			}
			fmt.Println("User saved successfully:", existingUser.Username)
		} else {
			user := models.User{
				Username: username,
				Email:    email,
			}

			if err := user.SetPassword(password); err != nil {
				log.Fatal("Error hashing password:", err)
			}

			if err := db.Create(&user).Error; err != nil {
				log.Fatal("Error creating user:", err)
			}
			fmt.Println("User created successfully:", user.Username)
		}

	},
}
