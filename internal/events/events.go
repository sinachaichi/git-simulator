// Package events defines what the lower layers report while a command
// runs. X-ray mode collects these events and prints them.
package events

// Kind says what happened.
type Kind string

// The kinds of event.
const (
	ObjectWritten  Kind = "object.written"  // a new object was stored
	ObjectExists   Kind = "object.exists"   // the object was already stored
	RefUpdated     Kind = "ref.updated"     // a branch or tag moved
	RefDeleted     Kind = "ref.deleted"     // a branch or tag was removed
	HeadSet        Kind = "head.set"        // HEAD now points somewhere else
	ReflogAppended Kind = "reflog.appended" // a line was added to a reflog
	IndexAdded     Kind = "index.added"     // a path entered the index
	IndexModified  Kind = "index.modified"  // a path in the index got new content
	IndexRemoved   Kind = "index.removed"   // a path left the index
	IndexConflict  Kind = "index.conflict"  // a path got conflict stages
	FileWritten    Kind = "file.written"    // a file was written in the working tree
	FileRemoved    Kind = "file.removed"    // a file was removed from the working tree
	StateWritten   Kind = "state.written"   // a state file such as MERGE_HEAD was written
	StateRemoved   Kind = "state.removed"   // a state file was removed
)

// Event is one thing that happened. Which fields are set depends on Kind.
type Event struct {
	Kind    Kind
	Subject string // object id, ref name or path
	Old     string
	New     string
	Detail  string // object type and size, reflog message, file mode, ...
}

// Sink receives events. A nil Sink means nobody is listening.
type Sink interface {
	Emit(Event)
}

// Recorder is a Sink that keeps the events in order.
type Recorder struct {
	events []Event
}

// Emit implements Sink.
func (r *Recorder) Emit(e Event) { r.events = append(r.events, e) }

// Events returns what was recorded so far.
func (r *Recorder) Events() []Event { return append([]Event(nil), r.events...) }

// Reset forgets all recorded events.
func (r *Recorder) Reset() { r.events = nil }
