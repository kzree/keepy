package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
)

func TestVaultDatabaseCopyPreservesDataAndIsolatesMutations(t *testing.T) {
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprintf("KDBX%d", version), func(t *testing.T) {
			source := richTestDatabase(t, version)
			if version == 4 {
				k := source.Header.FileHeaders.KdfParameters
				k.RawData = testVariantDictionary("raw", "parameters")
				k.SecretKey = []byte("secret key")
				k.AssocData = []byte("associated data")
			}
			before := snapshotTestDatabase(t, source)
			copied := copyVaultDatabase(source)
			require.Equal(t, source, copied, "copy must preserve identities, flags, nils, and all field values")
			entry := &copied.Content.Root.Groups[0].Groups[0].Entries[0]
			setEntryValue(entry, PasswordKey, "changed password")
			entry.Histories[0].Entries[0].Get(PasswordKey).Value.Content = "changed history"
			entry.CustomData[0].Value = "changed custom data"
			entry.AutoType.Associations[0].Window = "changed window"
			entry.Binaries[0].Value.ID = 123
			entry.Times.CreationTime.Time = entry.Times.CreationTime.Time.Add(time.Hour)
			assert.Same(t, entry.Times.CreationTime, entry.Times.LastModificationTime, "timestamp aliases stay inside the copy")
			copied.Content.Root.Groups[0].Groups[0].Name = "changed group"
			copied.Content.Root.Groups[0].Times.CreationTime.Formatted = false
			copied.Content.Root.DeletedObjects[0].DeletionTime.Formatted = false
			copied.Content.Meta.CustomIcons[0].Data = "changed icon"
			copied.Content.Meta.CustomData[0].Value = "changed plugin data"
			copied.Content.RawData[0] ^= 0xff
			copied.Header.RawData[0] ^= 0xff
			copied.Header.Signature.MinorVersion++
			copied.Header.FileHeaders.MasterSeed[0] ^= 0xff
			copied.Header.FileHeaders.CipherID[0] ^= 0xff
			copied.Credentials.Passphrase[0] ^= 0xff
			copied.Credentials.Key[0] ^= 0xff
			copied.Credentials.Windows[0] ^= 0xff
			copied.Options.ValidateHashes = false
			copied.Hashes.Sha256[0]++
			if version == 4 {
				copied.Content.InnerHeader.InnerRandomStreamKey[0] ^= 0xff
				k := copied.Header.FileHeaders.KdfParameters
				k.UUID[0] ^= 0xff
				k.SecretKey[0] ^= 0xff
				k.AssocData[0] ^= 0xff
				k.RawData.Items[0].Name[0] ^= 0xff
				k.RawData.Items[0].Value[0] ^= 0xff
				copied.Header.FileHeaders.PublicCustomData.Items[0].Value[0] ^= 0xff
			} else {
				copied.Header.FileHeaders.TransformSeed[0] ^= 0xff
				copied.Header.FileHeaders.ProtectedStreamKey[0] ^= 0xff
			}
			copied.FindBinary(0).Content[0] ^= 0xff
			require.NoError(t, copied.FindBinary(0).SetContent([]byte{0xfe, 0x00, 0x03}))
			assert.Equal(t, before, snapshotTestDatabase(t, source), "mutations must not escape to the source")
			originalBinary, err := source.FindBinary(0).GetContentBytes()
			require.NoError(t, err)
			assert.Equal(t, []byte{0xff, 0x00, 0x01, 0x02}, originalBinary)
			if version == 4 {
				assert.Equal(t, []byte{0xfe, 0x00, 0x03}, copied.FindBinary(0).Content, "private KDBX4 flag must survive the copy")
			}
		})
	}
}

func TestVaultDatabaseCopyPreservesPrivateGroupOrderingAndEmptyValues(t *testing.T) {
	assert.Nil(t, copyVaultDatabase(nil))
	var group gokeepasslib.Group
	require.NoError(t, xml.Unmarshal([]byte(`<Group><Name>Parent</Name><Group><Name>Child</Name></Group><Entry><String><Key>Title</Key><Value>Entry</Value></String></Entry></Group>`), &group))
	source := gokeepasslib.NewDatabase()
	source.Credentials = nil
	source.Content.Root.Groups = []gokeepasslib.Group{group}
	source.Content.Meta.CustomData = []gokeepasslib.CustomData{}
	source.Header.FileHeaders.PublicCustomData = &gokeepasslib.VariantDictionary{Items: []*gokeepasslib.VariantDictionaryItem{nil}}
	require.Equal(t, source, copyVaultDatabase(source))
}

func TestEncodingCopyLeavesUnlockedDatabasesUntouched(t *testing.T) {
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprintf("KDBX%d", version), func(t *testing.T) {
			live := richTestDatabase(t, version)
			liveBefore := snapshotTestDatabase(t, live)
			working := copyVaultDatabase(live)
			entry := &working.Content.Root.Groups[0].Groups[0].Entries[0]
			setEntryValue(entry, PasswordKey, "updated secret")
			workingBefore := snapshotTestDatabase(t, working)
			encoding, err := encodingVaultCopy(working)
			require.NoError(t, err)
			assert.NotEqual(t, "updated secret", encoding.Content.Root.Groups[0].Groups[0].Entries[0].GetPassword())
			var output bytes.Buffer
			require.NoError(t, gokeepasslib.NewEncoder(&output).Encode(encoding))
			assert.Equal(t, liveBefore, snapshotTestDatabase(t, live))
			assert.Equal(t, workingBefore, snapshotTestDatabase(t, working), "encoder side effects stay in the encoding copy")
			decoded := decodeTestDatabase(t, output.Bytes(), live.Credentials)
			result := decoded.Content.Root.Groups[0].Groups[0].Entries[0]
			assert.Equal(t, entry.UUID, result.UUID)
			assert.Equal(t, "updated secret", result.GetPassword())
			assert.Equal(t, "custom secret", result.GetContent("Custom secret"))
			assert.Equal(t, "old secret", result.Histories[0].Entries[0].GetPassword())
			assert.Equal(t, entry.Histories[0].Entries[0].UUID, result.Histories[0].Entries[0].UUID)
			require.Len(t, result.CustomData, 1)
			assert.Equal(t, entry.CustomData[0].Key, result.CustomData[0].Key)
			assert.Equal(t, entry.CustomData[0].Value, result.CustomData[0].Value)
			assert.Equal(t, entry.AutoType.Associations, result.AutoType.Associations)
			assert.Equal(t, live.Content.Meta.CustomIcons, decoded.Content.Meta.CustomIcons)
			assert.Equal(t, live.Content.Root.DeletedObjects[0].UUID, decoded.Content.Root.DeletedObjects[0].UUID)
			binary, err := decoded.FindBinary(result.Binaries[0].Value.ID).GetContentBytes()
			require.NoError(t, err)
			assert.Equal(t, []byte{0xff, 0x00, 0x01, 0x02}, binary)
			require.NotNil(t, working.FindBinary(1), "encoder cleanup must not remove the working copy's unused attachments")
		})
	}
}

func TestFailedVaultMutationsDiscardWorkingDatabase(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete", "save"} {
		t.Run(operation, func(t *testing.T) {
			values := NewVaultEntry{Title: "Original", Password: "secret"}
			entry := values.ToKeepassEntry()
			vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root", Entries: []gokeepasslib.Entry{entry}})
			originalPath := vault.dbPath
			live := vault.db
			before := snapshotTestDatabase(t, live)
			// A missing parent guarantees an open failure on every platform.
			vault.dbPath = filepath.Join(t.TempDir(), "missing", "vault.kdbx")
			var err error
			switch operation {
			case "add":
				err = vault.AddNewEntry(NewVaultEntry{Title: "New", Password: "new secret"})
			case "update":
				err = vault.UpdateEntry(entry.UUID, NewVaultEntry{Title: "Updated", Password: "updated secret"})
			case "delete":
				err = vault.DeleteEntry(entry.UUID)
			case "save":
				err = vault.LockAndSave()
			}
			require.Error(t, err)
			assert.Same(t, live, vault.db)
			assert.Equal(t, before, snapshotTestDatabase(t, vault.db))
			assertEntryPassword(t, vault, entry.UUID, "secret")
			vault.dbPath = originalPath
			require.NoError(t, vault.AddNewEntry(NewVaultEntry{Title: "Retry", Password: "retry secret"}))
			require.Len(t, vault.GetEntriesFlat(), 2, "only the explicitly retried addition is applied")
			reopened := authenticateTestVault(t, originalPath, "")
			require.Len(t, reopened.GetEntriesFlat(), 2)
			assertEntryPassword(t, reopened, entry.UUID, "secret")
		})
	}
}

func TestEncodingFailureLeavesLivePasswordsAndMetadataUnchanged(t *testing.T) {
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprintf("KDBX%d", version), func(t *testing.T) {
			live := richTestDatabase(t, version)
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			writeTestVaultDatabase(t, path, live)
			data := readTestFile(t, path)
			snapshot, err := newVaultPersistence().read(path)
			require.NoError(t, err)
			live.Credentials = nil // Fail after the encoder has locked fields and cleaned attachments.
			vault := &Vault{db: live, authenticated: true, dbPath: path, baseline: &snapshot.baseline}
			before := snapshotTestDatabase(t, live)
			err = vault.UpdateEntry(live.Content.Root.Groups[0].Groups[0].Entries[0].UUID, NewVaultEntry{Title: "Changed", Password: "changed"})
			require.Error(t, err)
			requireSaveErrorKind(t, err, SaveFailed)
			assert.Same(t, live, vault.db)
			assert.Equal(t, before, snapshotTestDatabase(t, live))
			assert.Equal(t, data, readTestFile(t, path))
			assertNoVaultTemps(t, path)
		})
	}
}

func TestMissingDeletionTimeIsRejectedBeforeWritingDatabase(t *testing.T) {
	vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root"})
	original := readTestFile(t, vault.dbPath)
	vault.db.Content.Root.DeletedObjects = []gokeepasslib.DeletedObjectData{{UUID: gokeepasslib.NewUUID()}}
	live := vault.db
	before := snapshotTestDatabase(t, live)
	require.EqualError(t, vault.LockAndSave(), "deleted object is missing its deletion time")
	assert.Same(t, live, vault.db)
	assert.Equal(t, before, snapshotTestDatabase(t, live))
	assert.Equal(t, original, readTestFile(t, vault.dbPath), "validation must precede opening the destination")
}
