package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/krixlion/dev_forum-article/pkg/event"
	"github.com/krixlion/dev_forum-article/pkg/event/dispatcher"
	"github.com/krixlion/dev_forum-article/pkg/logging"
	"github.com/krixlion/dev_forum-article/pkg/storage"
	"github.com/krixlion/dev_forum-proto/article_service/pb"

	"github.com/gofrs/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ArticleServer struct {
	pb.UnimplementedArticleServiceServer
	Storage    storage.CQRStorage
	Logger     logging.Logger
	Dispatcher *dispatcher.Dispatcher
}

func (s ArticleServer) Close() error {
	var errMsg string

	err := s.Storage.Close()
	if err != nil {
		errMsg = fmt.Sprintf("%s, failed to close storage: %s", errMsg, err)
	}

	if errMsg != "" {
		return errors.New(errMsg)
	}

	return nil
}

func (s ArticleServer) Create(ctx context.Context, req *pb.CreateArticleRequest) (*pb.CreateArticleResponse, error) {
	article := articleFromPB(req.GetArticle())
	id, err := uuid.NewV4()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Assign new UUID to article about to be created.
	article.Id = id.String()

	if err = s.Storage.Create(ctx, article); err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	s.Dispatcher.Publish(event.MakeEvent(event.ArticleCreated, article))

	return &pb.CreateArticleResponse{
		Id: id.String(),
	}, nil
}

func (s ArticleServer) Delete(ctx context.Context, req *pb.DeleteArticleRequest) (*pb.DeleteArticleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	id := req.GetId()

	if err := s.Storage.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}

	s.Dispatcher.Publish(event.MakeEvent(event.ArticleDeleted, id))

	return &pb.DeleteArticleResponse{}, nil
}

func (s ArticleServer) Update(ctx context.Context, req *pb.UpdateArticleRequest) (*pb.UpdateArticleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	article := articleFromPB(req.GetArticle())

	if err := s.Storage.Update(ctx, article); err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	s.Dispatcher.Publish(event.MakeEvent(event.ArticleUpdated, article))

	return &pb.UpdateArticleResponse{}, nil
}

func (s ArticleServer) Get(ctx context.Context, req *pb.GetArticleRequest) (*pb.GetArticleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	article, err := s.Storage.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get article: %v", err)
	}

	return &pb.GetArticleResponse{
		Article: &pb.Article{
			Id:        article.Id,
			UserId:    article.UserId,
			Title:     article.Title,
			Body:      article.Body,
			CreatedAt: timestamppb.New(article.CreatedAt),
			UpdatedAt: timestamppb.New(article.UpdatedAt),
		},
	}, err
}

func (s ArticleServer) GetStream(req *pb.GetArticlesRequest, stream pb.ArticleService_GetStreamServer) error {
	ctx, cancel := context.WithTimeout(stream.Context(), time.Second*10)
	defer cancel()

	articles, err := s.Storage.GetMultiple(ctx, req.GetOffset(), req.GetLimit())
	if err != nil {
		return err
	}

	for _, v := range articles {
		select {
		case <-ctx.Done():
			return nil
		default:
			article := pb.Article{
				Id:        v.Id,
				UserId:    v.UserId,
				Title:     v.Title,
				Body:      v.Body,
				CreatedAt: timestamppb.New(v.CreatedAt),
				UpdatedAt: timestamppb.New(v.UpdatedAt),
			}

			err := stream.Send(&article)
			if err != nil {
				return err
			}
		}
	}
	return nil
}