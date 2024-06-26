package transfer

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Processor is a transfer processor.
type Processor struct {
	handler *Pktline
	backend Backend
	logger  Logger
}

// NewProcessor creates a new transfer processor.
func NewProcessor(line *Pktline, backend Backend, logger Logger) *Processor {
	if logger == nil {
		logger = new(noopLogger)
	}
	return &Processor{
		handler: line,
		backend: backend,
		logger:  logger,
	}
}

// Version returns the version of the transfer protocol.
func (p *Processor) Version() (Status, error) {
	_, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, err
	}
	return NewSuccessStatusWithArgs([]string{}), nil
}

// Error returns a transfer protocol error.
func (p *Processor) Error(code uint32, message string, args ...string) (Status, error) {
	return NewStatusWithArgs(code, []string{message}, args...), nil
}

// ReadBatch reads a batch request.
func (p *Processor) ReadBatch(op string, args Args) ([]BatchItem, error) {
	data, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	hashAlgo := args[HashAlgoKey]
	switch hashAlgo {
	case "", "sha256":
	default:
		return nil, fmt.Errorf("%w: %s", ErrNotAllowed, fmt.Sprintf("unsupported hash algorithm: %s", hashAlgo))
	}
	p.logger.Log("read batch", "operation", op, "args-len", len(args), "args", args, "data-len", len(data), "data", data)
	items := make([]BatchItem, 0)
	for _, line := range data {
		if line == "" {
			return nil, ErrInvalidPacket
		}
		parts := strings.Split(line, " ")
		if len(parts) < 2 || parts[1] == "" {
			return nil, ErrParseError
		}
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid integer, got: %q", ErrParseError, parts[1])
		}
		var oidArgs Args
		if len(parts) > 2 {
			oidArgs, err = ParseArgs(parts[2:])
			if err != nil {
				return nil, fmt.Errorf("%w: %s", ErrParseError, err)
			}
		}
		item := BatchItem{
			Pointer: Pointer{
				Oid:  parts[0],
				Size: size,
			},
			Args: oidArgs,
		}
		items = append(items, item)
	}
	p.logger.Log("batch items", "items", items)
	its, err := p.backend.Batch(op, items, args)
	if err != nil {
		return nil, err
	}
	p.logger.Log("batch items", "items", items)
	return its, nil
}

// BatchData writes batch data to the transfer protocol.
func (p *Processor) BatchData(op string, presentAction string, missingAction string) (Status, error) {
	ar, err := p.handler.ReadPacketListToDelim()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	args, err := ParseArgs(ar)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	batch, err := p.ReadBatch(op, args)
	if err != nil {
		return nil, err
	}
	oids := make([]string, 0)
	for _, item := range batch {
		action := missingAction
		if item.Present {
			action = presentAction
		}
		line := fmt.Sprintf("%s %s", item.Pointer, action)
		if len(item.Args) > 0 {
			line = fmt.Sprintf("%s %s", line, item.Args)
		}
		oids = append(oids, line)
	}
	return NewSuccessStatus(oids...), nil
}

// UploadBatch writes upload data to the transfer protocol.
func (p *Processor) UploadBatch() (Status, error) {
	return p.BatchData(UploadOperation, "noop", "upload")
}

// DownloadBatch writes download data to the transfer protocol.
func (p *Processor) DownloadBatch() (Status, error) {
	return p.BatchData(DownloadOperation, "download", "noop")
}

// SizeFromArgs returns the size from the given args.
func SizeFromArgs(args Args) (int64, error) {
	size, ok := args[SizeKey]
	if !ok {
		return 0, fmt.Errorf("missing required size header")
	}
	n, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: %w", err)
	}
	return n, nil
}

// PutObject writes an object ID to the transfer protocol.
func (p *Processor) PutObject(oid string) (Status, error) {
	ar, err := p.handler.ReadPacketListToDelim()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	args, err := ParseArgs(ar)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	expectedSize, err := SizeFromArgs(args)
	if err != nil {
		return nil, err
	}
	r := p.handler.Reader()
	rdr := NewHashingReader(r, sha256.New())
	state, err := p.backend.StartUpload(oid, rdr, args)
	if err != nil {
		return nil, err
	}
	defer state.Close() // nolint: errcheck
	actualSize := rdr.Size()
	if actualSize != expectedSize {
		err := fmt.Errorf("invalid size, expected %d, got %d", expectedSize, actualSize)
		if actualSize > expectedSize {
			err = fmt.Errorf("%w: %s", ErrExtraData, err)
		} else {
			err = fmt.Errorf("%w: %s", ErrMissingData, err)
		}
		return nil, err
	}
	if actualOid := rdr.Oid(); actualOid != oid {
		return nil, fmt.Errorf("%w: %s", ErrCorruptData, fmt.Sprintf("invalid object ID, expected %s, got %s", oid, actualOid))
	}
	if err := p.backend.FinishUpload(state, args); err != nil {
		return nil, err
	}
	return SuccessStatus(), nil
}

// VerifyObject verifies an object ID.
func (p *Processor) VerifyObject(oid string) (Status, error) {
	ar, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	args, err := ParseArgs(ar)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	return p.backend.Verify(oid, args)
}

// GetObject writes an object ID to the transfer protocol.
func (p *Processor) GetObject(oid string) (Status, error) {
	ar, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	args, err := ParseArgs(ar)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	r, err := p.backend.Download(oid, args)
	if errors.Is(err, fs.ErrNotExist) {
		return NewStatus(StatusNotFound, fmt.Sprintf("object %s not found", oid)), nil
	}
	if err != nil {
		return nil, err
	}
	info, err := r.Stat()
	if err != nil {
		return nil, err
	}
	return NewSuccessStatusWithReader(r, fmt.Sprintf("size=%d", info.Size())), nil
}

// Lock writes a lock to the transfer protocol.
func (p *Processor) Lock() (Status, error) {
	data, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	args, err := ParseArgs(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	path := args[PathKey]
	refname := args[RefnameKey]
	if path == "" {
		return nil, fmt.Errorf("%w: %s", ErrMissingData, "path and refname are required")
	}
	lockBackend := p.backend.LockBackend(args)
	retried := false
	for {
		lock, err := lockBackend.Create(path, refname)
		if errors.Is(err, ErrConflict) {
			p.logger.Log("lock conflict")
			if lock == nil {
				lock, err = lockBackend.FromPath(path)
				if err != nil {
					p.logger.Log("lock conflict, but no lock found")
					if retried {
						p.logger.Log("lock conflict, but no lock found, and retried")
						return nil, err
					}
					retried = true
					continue
				}
			}
			return NewStatusWithArgs(StatusConflict, []string{"conflict"}, lock.AsArguments()...), nil
		}
		if err != nil {
			p.logger.Log("failed to create lock", "err", err)
			return nil, err
		}
		p.logger.Log("lock success", "lock", lock)
		return NewStatusWithArgs(StatusCreated, nil, lock.AsArguments()...), nil
	}
	// unreachable
}

// ListLocksForPath lists locks for a path. cursor can be empty.
func (p *Processor) ListLocksForPath(path string, cursor string, useOwnerID bool, args map[string]string) (Status, error) {
	lock, err := p.backend.LockBackend(args).FromPath(path)
	if err != nil {
		return nil, err
	}
	if (lock == nil && cursor == "") ||
		(lock.ID() < cursor) {
		return p.Error(StatusNotFound, fmt.Sprintf("lock for path %s not found", path))
	}
	spec, err := lock.AsLockSpec(useOwnerID)
	if err != nil {
		return nil, err
	}
	return NewSuccessStatus(spec...), nil
}

// ListLocks lists locks.
func (p *Processor) ListLocks(useOwnerID bool) (Status, error) {
	ar, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}

	args, err := ParseArgs(ar)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}

	limit, _ := strconv.Atoi(args[LimitKey])
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		// Try to avoid DoS attacks.
		limit = 100
	}

	cursor := args[CursorKey]
	if path, ok := args[PathKey]; ok && path != "" {
		return p.ListLocksForPath(path, cursor, useOwnerID, args)
	}

	locks := make([]Lock, 0)
	lb := p.backend.LockBackend(args)
	nextCursor, err := lb.Range(cursor, limit, func(lock Lock) error {
		if len(locks) >= limit {
			// stop iterating when limit is reached.
			return io.EOF
		}
		if lock == nil {
			// skip nil locks
			return nil
		}
		p.logger.Log("adding lock", "path", lock.Path(), "id", lock.ID())
		locks = append(locks, lock)
		return nil
	})
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
	}

	msgs := make([]string, 0, len(locks))
	for _, item := range locks {
		specs, err := item.AsLockSpec(useOwnerID)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, specs...)
	}

	dataArgs := []string{}
	if nextCursor != "" {
		dataArgs = append(dataArgs, fmt.Sprintf("next-cursor=%s", nextCursor))
	}

	return NewSuccessStatusWithArgs(msgs, dataArgs...), nil
}

// Unlock unlocks a lock.
func (p *Processor) Unlock(id string) (Status, error) {
	ar, err := p.handler.ReadPacketListToFlush()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	args, err := ParseArgs(ar)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err)
	}
	lock, err := p.backend.LockBackend(args).FromID(id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if lock == nil || errors.Is(err, ErrNotFound) {
		return p.Error(StatusNotFound, fmt.Sprintf("lock %s not found", id))
	}
	if err := lock.Unlock(); err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return p.Error(StatusNotFound, fmt.Sprintf("lock %s not found", id))
		case errors.Is(err, os.ErrPermission):
			return p.Error(StatusForbidden, fmt.Sprintf("lock %s not owned by you", id))
		default:
			return nil, err
		}
	}
	return NewSuccessStatusWithArgs(nil, lock.AsArguments()...), nil
}

// ProcessCommands processes commands from the transfer protocol.
func (p *Processor) ProcessCommands(op string) error {
	p.logger.Log("processing commands")
	for {
		pkt, err := p.handler.ReadPacketText()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		p.logger.Log("received packet", "packet", pkt)
		if pkt == "" {
			if err := p.handler.SendError(StatusBadRequest, "unknown command"); err != nil {
				p.logger.Log("failed to send pktline", "err", err)
			}
			continue
		}
		msgs := strings.Split(pkt, " ")
		if len(msgs) < 1 {
			if err := p.handler.SendError(StatusBadRequest, "no command provided"); err != nil {
				p.logger.Log("failed to send pktline", "err", err)
			}
			continue
		}
		p.logger.Log("received command", "command", msgs[0], "messages", msgs[1:])
		var status Status
		switch msgs[0] {
		case versionCommand:
			if len(msgs) > 0 && msgs[1] == Version {
				status, err = p.Version()
			} else {
				err = p.handler.SendError(StatusBadRequest, "unknown version")
			}
		case batchCommand:
			switch op {
			case UploadOperation:
				p.logger.Log("upload batch command received")
				status, err = p.UploadBatch()
			case DownloadOperation:
				p.logger.Log("download batch command received")
				status, err = p.DownloadBatch()
			default:
				err = p.handler.SendError(StatusBadRequest, "unknown operation")
			}
		case putObjectCommand:
			if len(msgs) > 1 {
				status, err = p.PutObject(msgs[1])
			} else {
				err = p.handler.SendError(StatusBadRequest, "bad request")
			}
		case verifyObjectCommand:
			if len(msgs) > 1 {
				status, err = p.VerifyObject(msgs[1])
			} else {
				err = p.handler.SendError(StatusBadRequest, "bad request")
			}
		case getObjectCommand:
			if len(msgs) > 1 {
				status, err = p.GetObject(msgs[1])
			} else {
				err = p.handler.SendError(StatusBadRequest, "bad request")
			}
		case lockCommand:
			status, err = p.Lock()
		case listLockCommand, "list-locks":
			switch op {
			case UploadOperation:
				status, err = p.ListLocks(true)
			case DownloadOperation:
				status, err = p.ListLocks(false)
			}
			p.logger.Log("list lock command", "status", status, "err", err)
		case unlockCommand:
			if len(msgs) > 1 {
				status, err = p.Unlock(msgs[1])
			} else {
				err = p.handler.SendError(StatusBadRequest, "unknown command")
			}
		case quitCommand:
			if err := p.handler.SendStatus(SuccessStatus()); err != nil {
				p.logger.Log("failed to send pktline", "err", err)
			}
			return nil
		default:
			err = p.handler.SendError(StatusBadRequest, "unknown command")
		}
		if err != nil {
			switch {
			case errors.Is(err, ErrExtraData),
				errors.Is(err, ErrParseError),
				errors.Is(err, ErrInvalidPacket),
				errors.Is(err, ErrCorruptData):
				if err := p.handler.SendError(StatusBadRequest, fmt.Errorf("error: %w", err).Error()); err != nil {
					p.logger.Log("failed to send pktline", "err", err)
				}
			case errors.Is(err, ErrNotAllowed):
				if err := p.handler.SendError(StatusMethodNotAllowed, fmt.Errorf("error: %w", err).Error()); err != nil {
					p.logger.Log("failed to send pktline", "err", err)
				}
			case errors.Is(err, ErrNotFound):
				if err := p.handler.SendError(StatusNotFound, fmt.Errorf("error: %w", err).Error()); err != nil {
					p.logger.Log("failed to send pktline", "err", err)
				}
			case errors.Is(err, ErrUnauthorized):
				if err := p.handler.SendError(StatusUnauthorized, fmt.Errorf("error: %w", err).Error()); err != nil {
					p.logger.Log("failed to send pktline", "err", err)
				}
			case errors.Is(err, ErrForbidden):
				if err := p.handler.SendError(StatusForbidden, fmt.Errorf("error: %w", err).Error()); err != nil {
					p.logger.Log("failed to send pktline", "err", err)
				}
			default:
				p.logger.Log("failed to process command", "err", err)
				if err := p.handler.SendError(StatusInternalServerError, "internal error"); err != nil {
					p.logger.Log("failed to send pktline", "err", err)
				}
			}
		}
		if status != nil {
			if err := p.handler.SendStatus(status); err != nil {
				p.logger.Log("failed to send pktline", "err", err)
			}
		}
		p.logger.Log("processed command")
	}
}
