package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// EventType represents a Gitlab event type.
type EventType string

// List of available event types.
const (
	EventTypeBuild        EventType = "Build Hook"
	EventTypeIssue        EventType = "Issue Hook"
	EventTypeJob          EventType = "Job Hook"
	EventTypeMergeRequest EventType = "Merge Request Hook"
	EventTypeNote         EventType = "Note Hook"
	EventTypePipeline     EventType = "Pipeline Hook"
	EventTypePush         EventType = "Push Hook"
	EventTypeTagPush      EventType = "Tag Push Hook"
	EventTypeWikiPage     EventType = "Wiki Page Hook"
)

const (
	noteableTypeCommit       = "Commit"
	noteableTypeMergeRequest = "MergeRequest"
	noteableTypeIssue        = "Issue"
	noteableTypeSnippet      = "Snippet"
)

type noteEvent struct {
	ObjectKind       string `json:"object_kind"`
	ObjectAttributes struct {
		NoteableType string `json:"noteable_type"`
	} `json:"object_attributes"`
}

const eventTypeHeader = "X-Gitlab-Event"

// WebhookEventType returns the event type for the given request.
func WebhookEventType(r *http.Request) EventType {
	return EventType(r.Header.Get(eventTypeHeader))
}

// ParseWebhook parses the event payload. For recognized event types, a
// value of the corresponding struct type will be returned. An error will
// be returned for unrecognized event types.
//
// Example usage:
//
// func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//     payload, err := ioutil.ReadAll(r.Body)
//     if err != nil { ... }
//     event, err := gitlab.ParseWebhook(gitlab.WebhookEventType(r), payload)
//     if err != nil { ... }
//     switch event := event.(type) {
//     case *gitlab.PushEvent:
//         processPushEvent(event)
//     case *gitlab.MergeEvent:
//         processMergeEvent(event)
//     ...
//     }
// }
//
func ParseWebhook(eventType EventType, payload []byte) (event interface{}, err error) {
	switch eventType {
	case EventTypeBuild:
		event = &BuildEvent{}
	case EventTypeIssue:
		event = &IssueEvent{}
	case EventTypeJob:
		event = &JobEvent{}
	case EventTypeMergeRequest:
		event = &MergeEvent{}
	case EventTypePipeline:
		event = &PipelineEvent{}
	case EventTypePush:
		event = &PushEvent{}
	case EventTypeTagPush:
		event = &TagEvent{}
	case EventTypeWikiPage:
		event = &WikiPageEvent{}
	case EventTypeNote:
		note := &noteEvent{}
		err := json.Unmarshal(payload, note)
		if err != nil {
			return nil, err
		}

		if note.ObjectKind != "note" {
			return nil, fmt.Errorf("unexpected object kind %s", note.ObjectKind)
		}

		switch note.ObjectAttributes.NoteableType {
		case noteableTypeCommit:
			event = &CommitCommentEvent{}
		case noteableTypeMergeRequest:
			event = &MergeCommentEvent{}
		case noteableTypeIssue:
			event = &IssueCommentEvent{}
		case noteableTypeSnippet:
			event = &SnippetCommentEvent{}
		default:
			return nil, fmt.Errorf("unexpected noteable type %s", note.ObjectAttributes.NoteableType)
		}

	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}
