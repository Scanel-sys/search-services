package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/kljensen/snowball"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "yadro.com/course/proto/words"
)

const maxPhraseSize = 4 * 1024
const averageRequestWords = 4

type server struct {
	wordspb.UnimplementedWordsServer
}

var wordRegexp = regexp.MustCompile(`[[:alnum:]]+`)

var stopWords = map[string]struct{}{
	"a": {}, "about": {}, "above": {}, "after": {}, "again": {}, "against": {}, "all": {},
	"am": {}, "an": {}, "and": {}, "any": {}, "are": {}, "as": {}, "at": {},

	"be": {}, "because": {}, "been": {}, "before": {}, "being": {}, "below": {}, "between": {},
	"both": {}, "but": {}, "by": {},

	"can": {}, "could": {}, "couldn": {},

	"did": {}, "didn": {}, "do": {}, "does": {}, "doesn": {}, "doing": {}, "don": {}, "down": {},
	"during": {},

	"each": {},

	"few": {}, "for": {}, "from": {}, "further": {},

	"had": {}, "hadn": {}, "has": {}, "hasn": {}, "have": {}, "haven": {}, "having": {},
	"he": {}, "her": {}, "here": {}, "hers": {}, "herself": {},
	"him": {}, "himself": {}, "his": {}, "how": {},

	"i": {}, "if": {}, "in": {}, "into": {}, "is": {}, "isn": {}, "it": {}, "its": {}, "itself": {},

	"just": {},

	"ma": {}, "me": {}, "more": {}, "most": {}, "mustn": {}, "my": {}, "myself": {},

	"no": {}, "nor": {}, "not": {}, "now": {},

	"of": {}, "off": {}, "on": {}, "once": {}, "only": {}, "or": {}, "other": {},
	"our": {}, "ours": {}, "ourselves": {}, "out": {}, "over": {}, "own": {},

	"s": {}, "same": {}, "shan": {}, "she": {}, "should": {}, "shouldn": {}, "so": {}, "some": {}, "such": {},

	"t": {}, "than": {}, "that": {}, "the": {}, "their": {}, "theirs": {}, "them": {},
	"themselves": {}, "then": {}, "there": {}, "these": {}, "they": {}, "this": {}, "those": {},
	"through": {}, "to": {}, "too": {},

	"under": {}, "until": {}, "up": {},

	"very": {},

	"was": {}, "wasn": {}, "we": {}, "were": {}, "weren": {}, "what": {}, "when": {},
	"where": {}, "which": {}, "while": {}, "who": {}, "whom": {}, "why": {}, "will": {},
	"with": {}, "won": {}, "would": {}, "wouldn": {},

	"you": {}, "your": {}, "yours": {}, "yourself": {}, "yourselves": {},
}

func (s *server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *server) Norm(ctx context.Context, in *wordspb.WordsRequest) (*wordspb.WordsReply, error) {

	if len(in.Phrase) > maxPhraseSize {
		return nil, status.Error(codes.ResourceExhausted, "phrase too large")
	}

	words := normalize(ctx, in.Phrase)

	return &wordspb.WordsReply{Words: words}, nil
}

func normalize(ctx context.Context, phrase string) []string {

	words := wordRegexp.FindAllString(phrase, -1)

	result := make([]string, 0, averageRequestWords)
	seen := make(map[string]struct{}, averageRequestWords)

	for _, word := range words {

		select {
		case <-ctx.Done():
			return result
		default:
		}

		word = strings.ToLower(word)

		if _, ok := stopWords[word]; ok {
			continue
		}

		stemmed, err := snowball.Stem(word, "english", true)
		if err != nil {
			slog.Error("Error stemming word:", "word", word, "error", err)
			continue
		}

		if _, ok := seen[stemmed]; ok {
			continue
		}

		seen[stemmed] = struct{}{}
		result = append(result, stemmed)
	}

	return result
}

func main() {
	configPath := flag.String("config", "config.yaml", "config path")
	addrFlag := flag.String("address", "", "server address")

	flag.Parse()

	address, port, err := ParseServerConfig(*configPath, *addrFlag)

	if err != nil {
		log.Fatalf("Error parsing server config: %v", err)
	}
	slog.Info("server config",
		"address", address,
		"port", port,
	)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, &server{})
	reflection.Register(s)

	go func() {
		if err := s.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	<-ctx.Done()

	slog.Info("shutting down")

	s.GracefulStop()
}
