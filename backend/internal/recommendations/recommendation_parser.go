package recommendations

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"backend/internal/common"
)

type parsedRecommendation struct {
	title  string
	reason string
	id     int
}

func normalizeRecommendations(answer string, candidates []common.MovieResponse) ([]common.MovieResponse, string) {
	byID, byTitle := indexCandidates(candidates)
	recommended, entries := matchParsedRecommendations(parseRecommendations(answer), byID, byTitle)
	appendFallbackRecommendations(&recommended, &entries, candidates)
	return recommended, formatRecommendations(entries)
}

func indexCandidates(candidates []common.MovieResponse) (map[int]common.MovieResponse, map[string]common.MovieResponse) {
	byID := make(map[int]common.MovieResponse, len(candidates))
	byTitle := make(map[string]common.MovieResponse, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.TMDBID] = candidate
		byTitle[normalizeMovieTitle(candidate.Title)] = candidate
	}
	return byID, byTitle
}

func matchParsedRecommendations(parsed []parsedRecommendation, byID map[int]common.MovieResponse, byTitle map[string]common.MovieResponse) ([]common.MovieResponse, []parsedRecommendation) {
	recommended := make([]common.MovieResponse, 0, 3)
	entries := make([]parsedRecommendation, 0, 3)
	for _, item := range parsed {
		movie, ok := matchCandidate(item, byID, byTitle)
		if !ok || containsMovie(recommended, movie.TMDBID) {
			continue
		}
		recommended = append(recommended, movie)
		entries = append(entries, parsedRecommendation{title: movie.Title, reason: cleanReason(item.reason)})
		if len(recommended) == 3 {
			break
		}
	}
	return recommended, entries
}

func matchCandidate(item parsedRecommendation, byID map[int]common.MovieResponse, byTitle map[string]common.MovieResponse) (common.MovieResponse, bool) {
	if item.title != "" {
		if movie, ok := byTitle[normalizeMovieTitle(item.title)]; ok {
			return movie, true
		}
	}
	if item.id != 0 {
		movie, ok := byID[item.id]
		return movie, ok
	}
	return common.MovieResponse{}, false
}

func appendFallbackRecommendations(recommended *[]common.MovieResponse, entries *[]parsedRecommendation, candidates []common.MovieResponse) {
	for _, candidate := range candidates {
		if len(*recommended) == 3 {
			return
		}
		if containsMovie(*recommended, candidate.TMDBID) {
			continue
		}
		*recommended = append(*recommended, candidate)
		*entries = append(*entries, parsedRecommendation{
			title:  candidate.Title,
			reason: "This movie matches the themes and tone of your request.",
		})
	}
}

func formatRecommendations(entries []parsedRecommendation) string {
	var builder strings.Builder
	for i, entry := range entries {
		fmt.Fprintf(&builder, "%d. Title: %s\n   Reason: %s\n", i+1, entry.title, entry.reason)
	}
	return strings.TrimSpace(builder.String())
}

func parseRecommendations(answer string) []parsedRecommendation {
	marker := regexp.MustCompile(`(?m)^\s*\d+\.\s*Title:\s*`)
	locations := marker.FindAllStringIndex(answer, -1)
	recommendations := make([]parsedRecommendation, 0, len(locations))
	idPattern := regexp.MustCompile(`\[ID:(\d+)\]`)
	for i, location := range locations {
		end := len(answer)
		if i+1 < len(locations) {
			end = locations[i+1][0]
		}
		block := answer[location[1]:end]
		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}
		item := parsedRecommendation{title: strings.TrimSpace(lines[0])}
		for _, line := range lines[1:] {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(trimmed), "reason:") {
				item.reason = strings.TrimSpace(trimmed[len("Reason:"):])
				continue
			}
			if match := idPattern.FindStringSubmatch(trimmed); len(match) == 2 {
				item.id, _ = strconv.Atoi(match[1])
			}
		}
		if item.title != "" && item.reason != "" {
			recommendations = append(recommendations, item)
		}
	}
	return recommendations
}

func cleanReason(reason string) string {
	reason = regexp.MustCompile(`\*`).ReplaceAllString(reason, "")
	reason = regexp.MustCompile(`(?i)(?:the )?database (?:id|identifier)(?: for this movie)?\s*:\s*\d*`).ReplaceAllString(reason, "")
	return strings.TrimSpace(strings.Join(strings.Fields(reason), " "))
}

func normalizeMovieTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(title)), " "))
}

func containsMovie(candidates []common.MovieResponse, tmdbID int) bool {
	for _, movie := range candidates {
		if movie.TMDBID == tmdbID {
			return true
		}
	}
	return false
}
