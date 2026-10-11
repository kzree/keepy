package service

import (
	"errors"
	"slices"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

// copyVaultDatabase preserves database identities and format state, unlike the
// library's Entry.Clone/Group.Clone, which generate new UUIDs. Struct assignments
// preserve private scalar flags (binary format and group ordering); all mutable
// slices and pointers are then copied explicitly. Review this code when updating
// gokeepasslib's data model. The caller must prevent concurrent source mutation.
func copyVaultDatabase(source *gokeepasslib.Database) *gokeepasslib.Database {
	if source == nil {
		return nil
	}
	c := databaseCopier{times: make(map[*wrappers.TimeWrapper]*wrappers.TimeWrapper)}
	db := copyPointer(source)
	db.Options = copyPointer(source.Options)
	db.Credentials = copyPointer(source.Credentials)
	if db.Credentials != nil {
		db.Credentials.Passphrase = slices.Clone(source.Credentials.Passphrase)
		db.Credentials.Key = slices.Clone(source.Credentials.Key)
		db.Credentials.Windows = slices.Clone(source.Credentials.Windows)
	}
	db.Header = copyVaultHeader(source.Header)
	db.Hashes = copyPointer(source.Hashes)
	db.Content = copyPointer(source.Content)
	if db.Content != nil {
		db.Content.RawData = slices.Clone(source.Content.RawData)
		db.Content.InnerHeader = copyPointer(source.Content.InnerHeader)
		if db.Content.InnerHeader != nil {
			db.Content.InnerHeader.InnerRandomStreamKey = slices.Clone(source.Content.InnerHeader.InnerRandomStreamKey)
			db.Content.InnerHeader.Binaries = copyVaultBinaries(source.Content.InnerHeader.Binaries)
		}
		db.Content.Meta = c.meta(source.Content.Meta)
		db.Content.Root = copyPointer(source.Content.Root)
		if db.Content.Root != nil {
			db.Content.Root.Groups = c.groups(source.Content.Root.Groups)
			db.Content.Root.DeletedObjects = slices.Clone(source.Content.Root.DeletedObjects)
			for i := range db.Content.Root.DeletedObjects {
				db.Content.Root.DeletedObjects[i].DeletionTime = c.time(source.Content.Root.DeletedObjects[i].DeletionTime)
			}
		}
	}
	return db
}

func copyVaultHeader(source *gokeepasslib.DBHeader) *gokeepasslib.DBHeader {
	header := copyPointer(source)
	if header == nil {
		return nil
	}
	header.RawData = slices.Clone(source.RawData)
	header.Signature = copyPointer(source.Signature)
	header.FileHeaders = copyPointer(source.FileHeaders)
	if header.FileHeaders == nil {
		return header
	}
	f := header.FileHeaders
	f.Comment = slices.Clone(f.Comment)
	f.CipherID = slices.Clone(f.CipherID)
	f.MasterSeed = slices.Clone(f.MasterSeed)
	f.TransformSeed = slices.Clone(f.TransformSeed)
	f.EncryptionIV = slices.Clone(f.EncryptionIV)
	f.ProtectedStreamKey = slices.Clone(f.ProtectedStreamKey)
	f.StreamStartBytes = slices.Clone(f.StreamStartBytes)
	f.PublicCustomData = copyVaultDictionary(f.PublicCustomData)
	f.KdfParameters = copyPointer(f.KdfParameters)
	if f.KdfParameters != nil {
		k := f.KdfParameters
		k.RawData = copyVaultDictionary(k.RawData)
		k.UUID = slices.Clone(k.UUID)
		k.SecretKey = slices.Clone(k.SecretKey)
		k.AssocData = slices.Clone(k.AssocData)
	}
	return header
}

func copyVaultDictionary(source *gokeepasslib.VariantDictionary) *gokeepasslib.VariantDictionary {
	dict := copyPointer(source)
	if dict == nil {
		return nil
	}
	dict.Items = slices.Clone(source.Items)
	for i, item := range source.Items {
		dict.Items[i] = copyPointer(item)
		if item != nil {
			dict.Items[i].Name = slices.Clone(item.Name)
			dict.Items[i].Value = slices.Clone(item.Value)
		}
	}
	return dict
}

func copyVaultBinaries(source gokeepasslib.Binaries) gokeepasslib.Binaries {
	binaries := slices.Clone(source)
	for i := range binaries {
		binaries[i].Content = slices.Clone(source[i].Content)
	}
	return binaries
}

// Timestamp aliases are common in library-created entries. Preserve those
// aliases inside each copy, while isolating the wrappers from the source.
type databaseCopier struct {
	times map[*wrappers.TimeWrapper]*wrappers.TimeWrapper
}

func (c *databaseCopier) time(source *wrappers.TimeWrapper) *wrappers.TimeWrapper {
	if source == nil {
		return nil
	}
	if copied, ok := c.times[source]; ok {
		return copied
	}
	// time.Time is a value type; its internal location data is immutable through
	// its public API and must not be cloned with reflection or serialization.
	copied := copyPointer(source)
	c.times[source] = copied
	return copied
}

func (c *databaseCopier) timeData(source gokeepasslib.TimeData) gokeepasslib.TimeData {
	result := source
	result.CreationTime = c.time(source.CreationTime)
	result.LastModificationTime = c.time(source.LastModificationTime)
	result.LastAccessTime = c.time(source.LastAccessTime)
	result.ExpiryTime = c.time(source.ExpiryTime)
	result.LocationChanged = c.time(source.LocationChanged)
	return result
}

func (c *databaseCopier) meta(source *gokeepasslib.MetaData) *gokeepasslib.MetaData {
	meta := copyPointer(source)
	if meta == nil {
		return nil
	}
	meta.SettingsChanged = c.time(source.SettingsChanged)
	meta.DatabaseNameChanged = c.time(source.DatabaseNameChanged)
	meta.DatabaseDescriptionChanged = c.time(source.DatabaseDescriptionChanged)
	meta.DefaultUserNameChanged = c.time(source.DefaultUserNameChanged)
	meta.MasterKeyChanged = c.time(source.MasterKeyChanged)
	meta.RecycleBinChanged = c.time(source.RecycleBinChanged)
	meta.EntryTemplatesGroupChanged = c.time(source.EntryTemplatesGroupChanged)
	meta.CustomIcons = slices.Clone(source.CustomIcons)
	meta.Binaries = copyVaultBinaries(source.Binaries)
	meta.CustomData = slices.Clone(source.CustomData)
	return meta
}

func (c *databaseCopier) groups(source []gokeepasslib.Group) []gokeepasslib.Group {
	groups := slices.Clone(source)
	for i := range groups {
		groups[i].Times = c.timeData(source[i].Times)
		groups[i].Entries = c.entries(source[i].Entries)
		groups[i].Groups = c.groups(source[i].Groups)
	}
	return groups
}

func (c *databaseCopier) entries(source []gokeepasslib.Entry) []gokeepasslib.Entry {
	entries := slices.Clone(source)
	for i := range entries {
		e := &entries[i]
		e.Times = c.timeData(source[i].Times)
		e.Values = slices.Clone(source[i].Values)
		e.AutoType.Associations = slices.Clone(source[i].AutoType.Associations)
		e.Binaries = slices.Clone(source[i].Binaries)
		e.CustomData = slices.Clone(source[i].CustomData)
		e.Histories = slices.Clone(source[i].Histories)
		for j := range e.Histories {
			e.Histories[j].Entries = c.entries(source[i].Histories[j].Entries)
		}
	}
	return entries
}

// copyPointer performs only a value copy. Callers explicitly copy any reference
// fields in the copied struct before exposing it as an isolated database.
func copyPointer[T any](source *T) *T {
	if source == nil {
		return nil
	}
	copied := *source
	return &copied
}

// encodingVaultCopy accepts an unlocked database and produces a separate locked
// copy. The encoder mutates that copy further (timestamps, headers, attachments).
// Neither the source nor the unlocked working database needs unlocking afterward.
func encodingVaultCopy(source *gokeepasslib.Database) (*gokeepasslib.Database, error) {
	if source == nil || source.Header == nil || source.Header.Signature == nil ||
		source.Header.FileHeaders == nil || source.Content == nil ||
		source.Content.Meta == nil || source.Content.Root == nil || len(source.Content.Root.Groups) == 0 {
		return nil, errors.New("database is incomplete")
	}
	if source.Header.IsKdbx4() && (source.Hashes == nil || source.Header.FileHeaders.KdfParameters == nil) {
		return nil, errors.New("KDBX4 database is incomplete")
	}
	// Unlike entry timestamps, the library normalizes deletion timestamps
	// without a nil check. Reject malformed objects before opening a save file.
	for _, deleted := range source.Content.Root.DeletedObjects {
		if deleted.DeletionTime == nil {
			return nil, errors.New("deleted object is missing its deletion time")
		}
	}
	// GetStreamManager can return nil for an invalid header. Check it before
	// calling the library's LockProtectedEntries, which assumes a manager exists.
	manager, err := source.GetStreamManager()
	if err != nil {
		return nil, err
	}
	if manager == nil {
		return nil, gokeepasslib.ErrUnsupportedStreamType
	}
	copied := copyVaultDatabase(source)
	if err := copied.LockProtectedEntries(); err != nil {
		return nil, err
	}
	return copied, nil
}
