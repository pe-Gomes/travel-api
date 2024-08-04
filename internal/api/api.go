package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"travel-api/internal/api/spec"
	"travel-api/internal/pgstore"

	openapi_types "github.com/discord-gophers/goapi-gen/types"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type store interface {
	GetParticipant(context.Context, uuid.UUID) (pgstore.Participant, error)
	ConfirmParticipant(context.Context, uuid.UUID) error
	CreateTripTx(context.Context, *pgxpool.Pool, spec.CreateTripRequest) (uuid.UUID, error)
	GetTrip(context.Context, uuid.UUID) (pgstore.Trip, error)
	UpdateTrip(context.Context, pgstore.UpdateTripParams) error
	GetTripActivities(context.Context, uuid.UUID) ([]pgstore.Activity, error)
	CreateActivity(context.Context, pgstore.CreateActivityParams) (uuid.UUID, error)
	InviteParticipantsToTrip(context.Context, []pgstore.InviteParticipantsToTripParams) (int64, error)
	GetParticipants(context.Context, uuid.UUID) ([]pgstore.Participant, error)
	ConfirmTrip(context.Context, pgstore.ConfirmTripParams) error
	CreateTripLink(context.Context, pgstore.CreateTripLinkParams) (uuid.UUID, error)
	GetTripLinks(context.Context, uuid.UUID) ([]pgstore.Link, error)
}

type mailer interface {
	SendConfirmTripEmailToTripOwner(uuid.UUID) error
	SendInvitationToParticipant(string, uuid.UUID) error
}

type API struct {
	store     store
	logger    *zap.Logger
	validator *validator.Validate
	pool      *pgxpool.Pool
	mailer    mailer
}

func NewAPI(pool *pgxpool.Pool, logger *zap.Logger, mailer mailer) API {
	validator := validator.New(validator.WithRequiredStructEnabled())
	return API{pgstore.New(pool), logger, validator, pool, mailer}
}

// Confirms a participant on a trip.

func (api *API) PatchParticipantsParticipantIDConfirm(w http.ResponseWriter, r *http.Request, participantID string) *spec.Response {
	id, err := uuid.Parse(string(participantID))
	if err != nil {
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	participant, err := api.store.GetParticipant(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PatchParticipantsParticipantIDConfirmJSON400Response(spec.Error{Message: "participante não encontrado"})
		}
		api.logger.Error("failed to get participant", zap.Error(err), zap.String("participant_id", participantID))
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente."})
	}

	if participant.IsConfirmed {
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(spec.Error{Message: "Participante já confirmado."})
	}

	if err := api.store.ConfirmParticipant(r.Context(), id); err != nil {

		api.logger.Error("failed to confirm participant", zap.Error(err), zap.String("participant_id", participantID))
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente."})
	}

	return spec.PatchParticipantsParticipantIDConfirmJSON204Response(nil)
}

// Create a new trip
// (POST /trips)
func (api *API) PostTrips(w http.ResponseWriter, r *http.Request) *spec.Response {
	var body spec.CreateTripRequest

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "JSON inválido"})
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "Invalid input:" + err.Error()})
	}

	tripID, err := api.store.CreateTripTx(r.Context(), api.pool, body)
	if err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "Falha ao criar a viagem, tente novamente."})
	}

	go func() {
		if err := api.mailer.SendConfirmTripEmailToTripOwner(tripID); err != nil {
			api.logger.Error("failed to send confirmation email on PostTrips: %w",
				zap.Error(err),
				zap.String("trip_id", tripID.String()))
		}
	}()

	return spec.PostTripsJSON201Response(spec.CreateTripResponse{TripID: tripID.String()})
}

// Get a trip details.
// (GET /trips/{tripId})
func (api *API) GetTripsTripID(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDJSON400Response(spec.Error{Message: "viagem não encontrada"})
		}
		return spec.GetTripsTripIDJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	return spec.GetTripsTripIDJSON200Response(spec.GetTripDetailsResponse{
		Trip: spec.GetTripDetailsResponseTripObj{
			ID:          trip.ID.String(),
			Destination: trip.Destination,
			StartsAt:    trip.StartsAt.Time,
			EndsAt:      trip.EndsAt.Time,
			IsConfirmed: trip.IsConfirmed,
		},
	})
}

// Update a trip.
// (PUT /trips/{tripId})
func (api *API) PutTripsTripID(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "viagem não encontrada"})
		}
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	var body spec.UpdateTripRequest

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "JSON inválido"})
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "Invalid input:" + err.Error()})
	}

	if err := api.store.UpdateTrip(r.Context(), pgstore.UpdateTripParams{
		Destination: body.Destination,
		StartsAt:    pgtype.Timestamp{Valid: true, Time: body.StartsAt},
		EndsAt:      pgtype.Timestamp{Valid: true, Time: body.EndsAt},
		ID:          id,
		IsConfirmed: trip.IsConfirmed,
	}); err != nil {
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	return spec.PutTripsTripIDJSON204Response(nil)
}

// Get a trip activities.
// (GET /trips/{tripId}/activities)
func (api *API) GetTripsTripIDActivities(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDActivitiesJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	activities, err := api.store.GetTripActivities(r.Context(), id)
	if err != nil {
		return spec.GetTripsTripIDActivitiesJSON400Response(spec.Error{Message: "algo deu errado, tente novamente"})
	}

	activityMap := make(map[time.Time][]spec.GetTripActivitiesResponseInnerArray)

	for _, activity := range activities {
		occursAt := activity.OccursAt.Time
		date := time.Date(
			occursAt.Year(),
			occursAt.Month(),
			occursAt.Day(),
			0, 0, 0, 0,
			occursAt.Location(),
		)

		innerActivity := spec.GetTripActivitiesResponseInnerArray{
			ID:       activity.ID.String(),
			OccursAt: occursAt,
			Title:    activity.Title,
		}

		activityMap[date] = append(activityMap[date], innerActivity)
	}

	var outerActivities []spec.GetTripActivitiesResponseOuterArray
	for date, innerActivities := range activityMap {
		outerActivities = append(outerActivities, spec.GetTripActivitiesResponseOuterArray{
			Activities: innerActivities,
			Date:       date,
		})
	}

	return spec.GetTripsTripIDActivitiesJSON200Response(spec.GetTripActivitiesResponse{
		Activities: outerActivities,
	})
}

// Create a trip activity.
// (POST /trips/{tripId}/activities)
func (api *API) PostTripsTripIDActivities(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	var body spec.PostTripsTripIDActivitiesJSONRequestBody

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "JSON inválido"})
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.GetTripsTripIDActivitiesJSON400Response(spec.Error{Message: "Invalid input:" + err.Error()})
	}

	activityID, err := api.store.CreateActivity(r.Context(), pgstore.CreateActivityParams{
		TripID:   id,
		Title:    body.Title,
		OccursAt: pgtype.Timestamp{Valid: true, Time: body.OccursAt},
	})
	if err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	return spec.PostTripsTripIDActivitiesJSON201Response(spec.CreateActivityResponse{ActivityID: activityID.String()})
}

// Confirm a trip and send e-mail invitations.
// (GET /trips/{tripId}/confirm)
func (api *API) GetTripsTripIDConfirm(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	participants, err := api.store.GetParticipants(r.Context(), id)
	if err != nil {
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "algo deu errado, tente novamente"})
	}

	if err := api.store.ConfirmTrip(r.Context(), pgstore.ConfirmTripParams{
		IsConfirmed: true,
	}); err != nil {
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "algo deu errado, tente novamente"})
	}

	for _, p := range participants {
		go func() {
			if err := api.mailer.SendInvitationToParticipant(string(p.Email), id); err != nil {
				api.logger.Error("Failed to send invitation to Participant on PostTripsTripIDInvites: %w",
					zap.Error(err),
					zap.String("participand_id", p.TripID.String()),
				)
			}
		}()
	}

	return spec.GetTripsTripIDConfirmJSON204Response(nil)
}

// Invite someone to the trip.
// (POST /trips/{tripId}/invites)
func (api *API) PostTripsTripIDInvites(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	var body spec.InviteParticipantRequest

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "JSON inválido"})
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "Invalid input:" + err.Error()})
	}

	participant := make([]pgstore.InviteParticipantsToTripParams, 1)

	participant[0] = pgstore.InviteParticipantsToTripParams{
		TripID: id,
		Email:  string(body.Email),
	}

	participantID, err := api.store.InviteParticipantsToTrip(r.Context(), participant)
	if err != nil {
		api.logger.Error("Failed to send invitation to Participant on PostTripsTripIDInvites: %w",
			zap.Error(err),
			zap.String("trip_id", id.String()))
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	go func() {
		if err := api.mailer.SendInvitationToParticipant(string(body.Email), id); err != nil {
			api.logger.Error("Failed to send invitation to Participant on PostTripsTripIDInvites: %w",
				zap.Error(err),
				zap.String("participand_id", fmt.Sprintf("%d", participantID)))
		}
	}()

	return spec.PostTripsTripIDInvitesJSON201Response(nil)
}

// Get a trip links.
// (GET /trips/{tripId}/links)
func (api *API) GetTripsTripIDLinks(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDLinksJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	links, err := api.store.GetTripLinks(r.Context(), id)
	if err != nil {
		return spec.GetTripsTripIDLinksJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	linksRes := make([]spec.GetLinksResponseArray, len(links))

	for i, link := range links {
		linksRes[i] = spec.GetLinksResponseArray{
			ID:    link.ID.String(),
			Title: link.Title,
			URL:   link.Url,
		}
	}

	return spec.GetTripsTripIDLinksJSON200Response(spec.GetLinksResponse{
		Links: linksRes,
	})
}

// Create a trip link.
// (POST /trips/{tripId}/links)
func (api *API) PostTripsTripIDLinks(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	var body spec.CreateLinkRequest

	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: "JSON inválido"})
	}

	if err := api.validator.Struct(body); err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: "Invalid input:" + err.Error()})
	}

	linkID, err := api.store.CreateTripLink(r.Context(), pgstore.CreateTripLinkParams{
		TripID: id,
		Title:  body.Title,
		Url:    body.URL,
	})
	if err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	return spec.PostTripsTripIDLinksJSON201Response(spec.CreateLinkResponse{
		LinkID: linkID.String(),
	})
}

// Get a trip participants.
// (GET /trips/{tripId}/participants)
func (api *API) GetTripsTripIDParticipants(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDParticipantsJSON400Response(spec.Error{Message: "uuid inválido"})
	}

	participants, err := api.store.GetParticipants(r.Context(), id)
	if err != nil {
		return spec.GetTripsTripIDParticipantsJSON400Response(spec.Error{Message: "Algo deu errado, tente novamente"})
	}

	participantsRes := make([]spec.GetTripParticipantsResponseArray, len(participants))

	for i, participant := range participants {
		participantsRes[i] = spec.GetTripParticipantsResponseArray{
			ID:          participant.ID.String(),
			Email:       openapi_types.Email(participant.Email),
			IsConfirmed: participant.IsConfirmed,
		}
	}

	return spec.GetTripsTripIDParticipantsJSON200Response(spec.GetTripParticipantsResponse{
		Participants: participantsRes,
	})
}
