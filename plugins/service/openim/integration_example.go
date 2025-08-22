// Package main demonstrates how to integrate the OpenIM service plugin with the Lynx framework
package main

import (
	"context"
	"log"
	"time"

	"github.com/go-lynx/lynx/plugins/service/openim"
	"github.com/go-lynx/lynx/plugins/service/openim/conf"
)

// IntegrationExample demonstrates the integration of OpenIM plugin with Lynx framework
func IntegrationExample() {
	log.Println("Starting OpenIM integration example...")

	// Get the OpenIM service instance from the plugin manager
	openimService := openim.GetOpenIMService()
	if openimService == nil {
		log.Fatal("Failed to get OpenIM service instance")
	}

	log.Println("OpenIM service instance retrieved successfully")

	// Get current configuration
	config := openimService.GetConfig()
	log.Printf("Current configuration - Server: %s, Storage: %s",
		config.Server.Addr, config.Storage.Type)

	// Register custom message handlers
	openimService.RegisterMessageHandler("text", handleTextMessage)
	openimService.RegisterMessageHandler("image", handleImageMessage)
	openimService.RegisterMessageHandler("custom", handleCustomMessage)

	// Register custom event handlers
	openimService.RegisterEventHandler("user_online", handleUserOnline)
	openimService.RegisterEventHandler("user_offline", handleUserOffline)

	// Send various types of messages
	sendMessageExamples(openimService)

	// Demonstrate group chat functionality
	demonstrateGroupChat(openimService)

	// Demonstrate configuration updates
	demonstrateConfigurationUpdate(openimService)

	log.Println("OpenIM integration example completed successfully")
}

// Message handlers
func handleTextMessage(ctx context.Context, msg *conf.Message) error {
	log.Printf("Text message handler: %s -> %s: %s",
		msg.Sender, msg.Receiver, msg.Content)
	return nil
}

func handleImageMessage(ctx context.Context, msg *conf.Message) error {
	log.Printf("Image message handler: %s -> %s: %s",
		msg.Sender, msg.Receiver, msg.Content)
	return nil
}

func handleCustomMessage(ctx context.Context, msg *conf.Message) error {
	log.Printf("Custom message handler: %s -> %s: %s",
		msg.Sender, msg.Receiver, msg.Content)
	return nil
}

// Event handlers
func handleUserOnline(ctx context.Context, event interface{}) error {
	log.Printf("User online event: %v", event)
	return nil
}

func handleUserOffline(ctx context.Context, event interface{}) error {
	log.Printf("User offline event: %v", event)
	return nil
}

// Send various message examples
func sendMessageExamples(openimService *openim.ServiceOpenIM) {
	ctx := context.Background()

	// Text message
	textMsg := &conf.Message{
		Type:      "text",
		Content:   "Hello from Lynx framework!",
		Sender:    "lynx_user",
		Receiver:  "recipient_user",
		Timestamp: time.Now().Unix(),
		Status:    "pending",
	}

	err := openimService.SendMessage(ctx, textMsg)
	if err != nil {
		log.Printf("Failed to send text message: %v", err)
	} else {
		log.Println("Text message sent successfully")
	}

	// Image message
	imageMsg := &conf.Message{
		Type:      "image",
		Content:   "https://lynx.go-lynx.com/logo.png",
		Sender:    "lynx_user",
		Receiver:  "recipient_user",
		Timestamp: time.Now().Unix(),
		Status:    "pending",
	}

	err = openimService.SendMessage(ctx, imageMsg)
	if err != nil {
		log.Printf("Failed to send image message: %v", err)
	} else {
		log.Println("Image message sent successfully")
	}

	// Custom message
	customMsg := &conf.Message{
		Type:      "custom",
		Content:   "Lynx framework integration test",
		Sender:    "lynx_user",
		Receiver:  "recipient_user",
		Timestamp: time.Now().Unix(),
		Status:    "pending",
	}

	err = openimService.SendMessage(ctx, customMsg)
	if err != nil {
		log.Printf("Failed to send custom message: %v", err)
	} else {
		log.Println("Custom message sent successfully")
	}
}

// Demonstrate group chat functionality
func demonstrateGroupChat(openimService *openim.ServiceOpenIM) {
	ctx := context.Background()

	// Group announcement
	announcement := &conf.Message{
		Type:      "text",
		Content:   "Welcome to the Lynx framework group!",
		Sender:    "admin",
		GroupID:   "lynx_group",
		Timestamp: time.Now().Unix(),
		Status:    "pending",
	}

	err := openimService.SendMessage(ctx, announcement)
	if err != nil {
		log.Printf("Failed to send group announcement: %v", err)
		return
	}

	log.Println("Group announcement sent successfully")

	// Simulate group members joining
	members := []string{"user1", "user2", "user3"}
	for _, member := range members {
		joinMsg := &conf.Message{
			Type:      "text",
			Content:   "Hello everyone!",
			Sender:    member,
			GroupID:   "lynx_group",
			Timestamp: time.Now().Unix(),
			Status:    "pending",
		}

		err = openimService.SendMessage(ctx, joinMsg)
		if err != nil {
			log.Printf("Failed to send join message for %s: %v", member, err)
		} else {
			log.Printf("Join message sent for %s", member)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Demonstrate configuration updates
func demonstrateConfigurationUpdate(openimService *openim.ServiceOpenIM) {
	// Get current configuration
	currentConfig := openimService.GetConfig()
	log.Printf("Current server address: %s", currentConfig.Server.Addr)

	// Create new configuration
	newConfig := &conf.OpenIM{
		Server: &conf.Server{
			Addr:       "localhost:10003",
			LogLevel:   "debug",
			APIVersion: "v3",
			ServerName: "Updated OpenIM Server",
		},
		Client: &conf.Client{
			UserID:  "updated_user",
			Timeout: 60 * time.Second,
		},
		Security: &conf.Security{
			TLSEnable:  true,
			AuthEnable: true,
		},
		Storage: &conf.Storage{
			Type: "mysql",
			Addr: "localhost:3306",
		},
	}

	// Apply new configuration
	err := openimService.Configure(newConfig)
	if err != nil {
		log.Printf("Failed to apply new configuration: %v", err)
		return
	}

	log.Println("New configuration applied successfully")

	// Verify configuration change
	updatedConfig := openimService.GetConfig()
	log.Printf("Updated server address: %s", updatedConfig.Server.Addr)
	log.Printf("Updated log level: %s", updatedConfig.Server.LogLevel)
	log.Printf("Updated server name: %s", updatedConfig.Server.ServerName)
}

// Main function for the integration example
func main() {
	log.Println("OpenIM Plugin Integration Example")
	log.Println("=================================")

	// Run the integration example
	IntegrationExample()

	log.Println("Integration example completed")
}
