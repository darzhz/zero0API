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
			errs := app.ExpandRecord(bookingData, []string{"UserId", "TicketID"}, nil)
			if len(errs) > 0 {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Failed to expand records",
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
			erros := app.ExpandRecords(attendees, []string{"user", "bookingId"}, nil)
			if len(erros) > 0 {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Failed to expand records",
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
					"message":   "Valid Ticket",
					"status":    "success",
					"checkedin": bookingData,
				})
			}

			// Already verified
			return c.JSON(http.StatusOK, map[string]any{
				"message":      "Already verified",
				"status":       "success",
				"alreadythere": attendees,
			})
		})
		e.Router.GET("/api/attendees/{eventId}", func(c *core.RequestEvent) error {
			// list of bookings with isAttended value
			eventId := c.Request.PathValue("eventId")

			// Get collections
			bookingCollection, _ := app.FindCollectionByNameOrId("Bookings")
			attendanceCollection, _ := app.FindCollectionByNameOrId("eventAttendees")

			// Find booking list
			bookings, err := app.FindRecordsByFilter(
				bookingCollection,
				fmt.Sprintf("EventId = '%s'", eventId),
				"",
				0, 0,
			)
			//expand record
			errs := app.ExpandRecords(bookings, []string{"UserId", "TicketID"}, nil)
			if len(errs) > 0 {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Failed to expand records",
				})
			}

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"message": "Error checking booking records",
					"status":  "failed",
				})
			}

			// Check if already verified
			attendees, err := app.FindRecordsByFilter(
				attendanceCollection,
				fmt.Sprintf("event = '%s'", eventId),
				"",
				0, 0,
			)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"message": "Error checking attendee records",
					"status":  "failed",
				})
			}
			//add a field to booking record called isAttended is bookingid exists in attendees
			// Build a map of attended booking IDs
			attendedMap := make(map[string]bool)
			for _, attendee := range attendees {
				if bid, ok := attendee.Get("bookingId").(string); ok {
					attendedMap[bid] = true
				}
			}

			// Add isAttended to each booking
			var finalBookings []map[string]any

			for _, booking := range bookings {
				id, ok := booking.Get("id").(string)
				if !ok {
					return c.JSON(http.StatusInternalServerError, map[string]any{
						"message": "Invalid booking data (id)",
						"status":  "failed",
					})
				}

				// Copy all booking fields into a new map
				bookingMap := make(map[string]any)
				for k, v := range booking.PublicExport() {
					bookingMap[k] = v
				}

				// Inject computed field
				bookingMap["isAttended"] = attendedMap[id]

				finalBookings = append(finalBookings, bookingMap)
			}

			return c.JSON(http.StatusOK, map[string]any{
				"message": "Success",
				"status":  "success",
				"data":    finalBookings,
			})

		})

		return e.Next()
	}
}
