package mailpit

import (
	"context"
	"fmt"
	"time"
	"travel-api/internal/pgstore"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wneessen/go-mail"
)

type store interface {
	GetTrip(context.Context, uuid.UUID) (pgstore.Trip, error)
}

type Mailpit struct {
	store store
}

func NewMailpit(pool *pgxpool.Pool) Mailpit {
	return Mailpit{pgstore.New(pool)}
}

func (mp Mailpit) SendConfirmTripEmailToTripOwner(tripID uuid.UUID) error {
	ctx := context.Background()
	trip, err := mp.store.GetTrip(ctx, tripID)
	if err != nil {
		return fmt.Errorf("mailpit: failed to get trip for SendConfirmTripToTripOwner: %w", err)
	}

	msg := mail.NewMsg()
	if err := msg.From("mailpit@travel.com"); err != nil {
		return fmt.Errorf("mailpit: failed to set From in email SendConfirmTripToTripOwner: %w", err)
	}

	if err := msg.To(trip.OwnerEmail); err != nil {
		return fmt.Errorf("mailpit: failed to set To in email SendConfirmTripToTripOwner: %w", err)
	}

	msg.Subject("Confirmação de viagem")
	msg.SetBodyString(mail.TypeTextPlain, fmt.Sprintf(`
		Olá, %s!

		A sua viagem para %s que começa no dia %s foi confirmada com sucesso!
		Clique no botão abaixo para ver mais detalhes sobre a sua viagem e confirmar sua presença.
		`, trip.OwnerName, trip.Destination, trip.StartsAt.Time.Format(time.DateOnly),
	))

	client, err := mail.NewClient("mailpit", mail.WithTLSPortPolicy(mail.NoTLS), mail.WithPort(1025))
	if err != nil {
		return fmt.Errorf("mailpit: failed to create email client to SendConfirmTripToTripOwner: %w", err)
	}

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("mailpit: failed to send email to SendConfirmTripToTripOwner: %w", err)
	}

	return nil
}

func (mp Mailpit) SendInvitationToParticipant(email string, tripID uuid.UUID) error {
	ctx := context.Background()
	trip, err := mp.store.GetTrip(ctx, tripID)
	if err != nil {
		return fmt.Errorf("mailpit: failed to get trip for SendInvitationToParticipant: %w", err)
	}
	msg := mail.NewMsg()

	if err := msg.From("mailpit@travel.com"); err != nil {
		return fmt.Errorf("mailpit: failed to set From email in SendInvitationToParticipant: %w", err)
	}

	if err := msg.To(email); err != nil {
		return fmt.Errorf("mailpit: failed to set To email in SendInvitationToParticipant: %w", err)
	}

	msg.Subject("Convite para viagem!")
	msg.SetBodyString(mail.TypeTextPlain, fmt.Sprintf(`
		Olás!

		Você foi convidado para participar da viagem para %s que começa no dia %s.

		Clique no botão abaixo para ver mais detalhes sobre a viagem e confirmar sua presença.
		`, trip.Destination, trip.StartsAt.Time.Format(time.DateOnly),
	))

	client, err := mail.NewClient("mailpit", mail.WithTLSPortPolicy(mail.NoTLS), mail.WithPort(1025))
	if err != nil {
		return fmt.Errorf("mailpit: failed to create email client to SendInvitationToParticipant: %w", err)
	}

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("mailpit: failed to send email to SendInvitationToParticipant: %w", err)
	}

	return nil
}
