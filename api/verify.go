package api

import (
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func SetupVerifyRoutes(app *pocketbase.PocketBase) func(*core.ServeEvent) error {
	return func(e *core.ServeEvent) error {
		e.Router.GET("/api/verify/{bookingId}", func(c *core.RequestEvent) error {
			bookingId := c.Request.PathValue("bookingId")

			// Get collections
			bookingCollection, _ := app.FindCollectionByNameOrId("Bookings")
			attendanceCollection, _ := app.FindCollectionByNameOrId("eventAttendees")

			// Find booking record
			bookingData, err := app.FindRecordById(bookingCollection, bookingId)
			if err != nil {
				return c.JSON(http.StatusNotAcceptable, map[string]any{
					"message": "Invalid Ticket",
					"status":  "failed",
				})
			}

			// Check if already verified
			attendees, err := app.FindRecordsByFilter(
				attendanceCollection,
				fmt.Sprintf("bookingId = '%s'", bookingId),
				"",
				0, 0,
			)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"message": "Error checking attendee records",
					"status":  "failed",
				})
			}

			if len(attendees) == 0 {
				// Not verified yet
				userId, ok1 := bookingData.Get("UserId").(string)
				eventId, ok2 := bookingData.Get("EventId").(string)

				if !ok1 || !ok2 {
					return c.JSON(http.StatusInternalServerError, map[string]any{
						"message": "Invalid booking data (UserId/EventId)",
						"status":  "failed",
					})
				}

				newRecord := core.NewRecord(attendanceCollection)
				newRecord.Set("bookingId", bookingId)
				newRecord.Set("user", userId)
				newRecord.Set("event", eventId)

				if err := app.Save(newRecord); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]any{
						"message": "Failed to save attendee record",
						"status":  "failed",
					})
				}

				return c.JSON(http.StatusOK, map[string]any{
					"message": "Valid Ticket",
					"status":  "success",
					"data":    bookingData,
				})
			}

			// Already verified
			return c.JSON(http.StatusOK, map[string]any{
				"message": "Already verified",
				"status":  "success",
				"data":    attendees,
			})
		})

		return e.Next()
	}
}
