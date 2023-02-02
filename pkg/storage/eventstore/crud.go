package eventstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/krixlion/dev_forum-article/pkg/entity"
	"github.com/krixlion/dev_forum-lib/event"
	"github.com/krixlion/dev_forum-lib/tracing"

	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
)

func addArticlesPrefix(v string) string {
	return fmt.Sprintf("%s-%s", "article", v)
}

func (db DB) Create(ctx context.Context, article entity.Article) error {
	ctx, span := db.tracer.Start(ctx, "esdb.Create")
	defer span.End()

	e := event.MakeEvent(event.ArticleCreated, article)
	data, err := json.Marshal(e)
	if err != nil {
		tracing.SetSpanErr(span, err)
		return err
	}

	eventData := esdb.EventData{
		ContentType: esdb.ContentTypeJson,
		EventType:   string(e.Type),
		Data:        data,
	}
	streamID := addArticlesPrefix(article.Id)

	_, err = db.client.AppendToStream(ctx, streamID, esdb.AppendToStreamOptions{}, eventData)
	if err != nil {
		tracing.SetSpanErr(span, err)
		return err
	}

	return nil
}

func (db DB) Update(ctx context.Context, article entity.Article) error {
	ctx, span := db.tracer.Start(ctx, "esdb.Update")
	defer span.End()

	e := event.MakeEvent(event.ArticleUpdated, article)
	data, err := json.Marshal(e)
	if err != nil {
		tracing.SetSpanErr(span, err)
		return err
	}

	lastEvent, err := db.lastRevision(ctx, article.Id)
	if err != nil {
		return err
	}

	appendOpts := esdb.AppendToStreamOptions{
		ExpectedRevision: esdb.Revision(lastEvent.OriginalEvent().EventNumber),
	}

	eventData := esdb.EventData{
		ContentType: esdb.ContentTypeJson,
		EventType:   string(e.Type),
		Data:        data,
	}
	streamID := addArticlesPrefix(article.Id)

	_, err = db.client.AppendToStream(ctx, streamID, appendOpts, eventData)
	if err != nil {
		tracing.SetSpanErr(span, err)
		return err
	}

	return nil
}

func (db DB) Delete(ctx context.Context, id string) error {
	ctx, span := db.tracer.Start(ctx, "esdb.Delete")
	defer span.End()

	e := event.MakeEvent(event.ArticleDeleted, id)
	data, err := json.Marshal(e)
	if err != nil {
		tracing.SetSpanErr(span, err)
		return err
	}

	eventData := esdb.EventData{
		ContentType: esdb.ContentTypeJson,
		EventType:   string(e.Type),
		Data:        data,
	}
	streamID := addArticlesPrefix(id)

	_, err = db.client.AppendToStream(ctx, streamID, esdb.AppendToStreamOptions{}, eventData)

	if err != nil {
		tracing.SetSpanErr(span, err)
		return err
	}

	return nil
}

func (db DB) lastRevision(ctx context.Context, articleId string) (*esdb.ResolvedEvent, error) {
	ctx, span := db.tracer.Start(ctx, "esdb.lastRevision")
	defer span.End()

	readOpts := esdb.ReadStreamOptions{
		Direction: esdb.Backwards,
		From:      esdb.End{},
	}

	streamID := addArticlesPrefix(articleId)

	stream, err := db.client.ReadStream(ctx, streamID, readOpts, 1)
	if err != nil {
		tracing.SetSpanErr(span, err)
		return nil, err
	}
	defer stream.Close()

	lastEvent, err := stream.Recv()
	if err != nil {
		tracing.SetSpanErr(span, err)
		return nil, err
	}

	return lastEvent, nil
}
