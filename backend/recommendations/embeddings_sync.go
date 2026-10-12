package recommendations

import (
	"context"
	"fmt"

	"backend/common"
)

const embeddingSyncPageSize = 100

// syncEmbeddings embeds every catalogue synopsis that has no stored embedding yet.
func (s *Service) syncEmbeddings(ctx context.Context) error {
	after := 0
	for {
		page, err := s.catalog.ListSynopses(ctx, after, embeddingSyncPageSize)
		if err != nil {
			return fmt.Errorf("list movie synopses: %w", err)
		}
		if len(page) == 0 {
			return nil
		}
		if err := s.embedMissing(ctx, page); err != nil {
			return err
		}
		after = page[len(page)-1].TMDBID
	}
}

func (s *Service) embedMissing(ctx context.Context, page []common.MovieSynopsis) error {
	ids := make([]int, len(page))
	for i, synopsis := range page {
		ids[i] = synopsis.TMDBID
	}
	existing, err := s.embeddings.Existing(ctx, ids)
	if err != nil {
		return err
	}
	for _, synopsis := range page {
		if existing[synopsis.TMDBID] {
			continue
		}
		if err := s.embedSynopsis(ctx, synopsis); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) embedSynopsis(ctx context.Context, synopsis common.MovieSynopsis) error {
	vector, err := s.embedding.Embed(ctx, synopsis.Synopsis)
	if err != nil {
		return fmt.Errorf("embed movie %d: %w", synopsis.TMDBID, err)
	}
	return s.embeddings.Save(ctx, synopsis.TMDBID, vector)
}
