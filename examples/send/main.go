// Send an SMS and show the balance. MACRASMS_SENDER_ID and MACRASMS_API_KEY must be set.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/macrasystems-star/macrasms-go"
)

func main() {
	c := macrasms.NewClient(os.Getenv("MACRASMS_SENDER_ID"), os.Getenv("MACRASMS_API_KEY"))
	ctx := context.Background()

	res, err := c.Send(ctx, []string{"0712345678"}, "Your order is ready for collection.")
	switch {
	case errors.Is(err, macrasms.ErrInsufficientBalance):
		log.Fatal("top up your MacraSMS credits first")
	case errors.Is(err, macrasms.ErrAuthentication):
		log.Fatal("check your sender ID and API key")
	case err != nil:
		log.Fatal(err)
	}
	fmt.Println("queued message", res.MessageID, "credits left:", res.BalanceRemaining)

	st, err := c.MessageStatus(ctx, res.MessageID)
	if err == nil {
		fmt.Println("status:", st.Status)
	}
}
