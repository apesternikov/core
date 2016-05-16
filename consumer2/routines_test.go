package consumer

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/cockroachdb/cockroach/util/encoding"
	etcd "github.com/coreos/etcd/client"
	gc "github.com/go-check/check"
	"github.com/stretchr/testify/mock"
	rocks "github.com/tecbot/gorocksdb"

	"github.com/pippio/consensus"
	"github.com/pippio/gazette/journal"
	"github.com/pippio/gazette/recoverylog"
	"github.com/pippio/gazette/topic"
)

type RoutinesSuite struct {
	keysAPI *consensus.MockKeysAPI
}

var (
	id8  = ShardID{"foo", 8}
	id12 = ShardID{"baz", 12}
	id30 = ShardID{"bar", 30}
	id42 = ShardID{"quux", 42}
)

func (s *RoutinesSuite) SetUpTest(c *gc.C) {
	s.keysAPI = new(consensus.MockKeysAPI)
}

func (s *RoutinesSuite) TestShardName(c *gc.C) {
	c.Check(id42.String(), gc.Equals, "shard-quux-042")
}

func (s *RoutinesSuite) TestHintsPath(c *gc.C) {
	c.Check(hintsPath(s.treeFixture().Key, id42), gc.Equals, "/foo/hints/shard-quux-042")
}

func (s *RoutinesSuite) TestLoadHints(c *gc.C) {
	runner := &Runner{RecoveryLogRoot: "path/to/recovery/logs/"}

	// Expect valid hints are found & loaded.
	hints, err := loadHintsFromEtcd(id12, runner, s.treeFixture())
	c.Check(err, gc.IsNil)
	c.Check(hints, gc.DeepEquals, s.hintsFixture())

	// Malformed hints.
	hints, err = loadHintsFromEtcd(id30, runner, s.treeFixture())
	c.Check(err, gc.ErrorMatches, "invalid character .*")

	// Missing hints.
	hints, err = loadHintsFromEtcd(id8, runner, s.treeFixture())
	c.Check(err, gc.IsNil)
	c.Check(hints, gc.DeepEquals, recoverylog.FSMHints{
		Log: "path/to/recovery/logs/shard-foo-008",
	})
}

func (s *RoutinesSuite) TestLoadOffsetsFromEtcd(c *gc.C) {
	offsets, err := loadOffsetsFromEtcd(s.treeFixture())
	c.Check(err, gc.IsNil)

	c.Check(offsets, gc.DeepEquals, map[journal.Name]int64{
		"journal/part-001":       42,
		"journal/part-002":       43,
		"other-journal/part-002": 44,
	})

	offsets, err = loadOffsetsFromEtcd(&etcd.Node{Key: "/foo", Dir: true})
	c.Check(err, gc.IsNil)
	c.Check(offsets, gc.IsNil)

	badTree := s.treeFixture()
	badTree.Nodes[1].Nodes[1].Nodes[0].Value = "invalid" // other-journal/part-002.

	offsets, err = loadOffsetsFromEtcd(badTree)
	c.Check(err, gc.ErrorMatches, "strconv.ParseInt: .*")
}

func (s *RoutinesSuite) TestStoreHintsToEtcd(c *gc.C) {
	hintsPath := "/foo/hints/shard-baz-012"
	shard012, _ := json.Marshal(s.hintsFixture())

	s.keysAPI.On("Set", mock.Anything, hintsPath, string(shard012),
		mock.Anything).Return(&etcd.Response{}, nil)

	storeHintsToEtcd(hintsPath, string(shard012), s.keysAPI)
	s.keysAPI.AssertExpectations(c)
}

func (s *RoutinesSuite) TestStoreOffsetsToEtcd(c *gc.C) {
	rootPath := "foo"
	offsets := make(map[journal.Name]int64)
	offsets["journal/part-001"] = 1000
	offsets["journal/part-002"] = 2000
	for k, v := range offsets {
		s.keysAPI.On("Set", mock.Anything, offsetPath(rootPath, k), strconv.FormatInt(v, 16),
			mock.Anything).Return(&etcd.Response{}, nil)
	}
	storeOffsetsToEtcd(rootPath, offsets, s.keysAPI)
	s.keysAPI.AssertExpectations(c)
}

func (s *RoutinesSuite) TestLoadAndStoreOffsetsToDB(c *gc.C) {
	path, err := ioutil.TempDir("", "routines-suite")
	c.Assert(err, gc.IsNil)
	defer func() { c.Check(os.RemoveAll(path), gc.IsNil) }()

	options := rocks.NewDefaultOptions()
	options.SetCreateIfMissing(true)
	defer options.Destroy()

	db, err := rocks.OpenDb(options, path)
	c.Assert(err, gc.IsNil)
	defer db.Close()

	wb := rocks.NewWriteBatch()
	wo := rocks.NewDefaultWriteOptions()
	ro := rocks.NewDefaultReadOptions()
	defer func() {
		wb.Destroy()
		wo.Destroy()
		ro.Destroy()
	}()

	offsets := map[journal.Name]int64{
		"journal/part-001":       42,
		"journal/part-002":       43,
		"other-journal/part-003": 44,
	}
	storeOffsetsToDB(wb, offsets)
	clearOffsets(offsets)
	c.Check(db.Write(wo, wb), gc.Equals, nil)

	// Expect |offsets| were Put to |wb| and then cleared.
	c.Check(wb.Count(), gc.Equals, 3)
	c.Check(offsets, gc.HasLen, 0)

	// Expect they're recovered from the database.
	recovered, err := loadOffsetsFromDB(db, ro)
	c.Check(err, gc.IsNil)
	c.Check(recovered, gc.DeepEquals, map[journal.Name]int64{
		"journal/part-001":       42,
		"journal/part-002":       43,
		"other-journal/part-003": 44,
	})

	markKey := func(suffix []byte) []byte {
		b := encoding.EncodeNullAscending(nil)
		b = encoding.EncodeStringAscending(b, "mark")
		b = append(b, suffix...)
		return b
	}

	// Test handling of a bad value encoding.
	cases := []struct {
		key, value []byte
		expect     string
	}{
		// Unexpected key encodings.
		{markKey([]byte("bad key")),
			encoding.EncodeVarintAscending(nil, 42), "did not find marker .*"},
		// Bad value encoding.
		{markKey(encoding.EncodeStringAscending(nil, "a/valid/journal")),
			[]byte("bad data"), "insufficient bytes to decode .*"},
	}

	for _, tc := range cases {
		c.Check(db.Put(wo, tc.key, tc.value), gc.IsNil)
		_, err = loadOffsetsFromDB(db, ro)
		c.Check(err, gc.ErrorMatches, tc.expect)

		c.Check(db.Delete(wo, tc.key), gc.IsNil) // Cleanup.
	}
}

func (s *RoutinesSuite) TestOffsetMerge(c *gc.C) {
	c.Check(mergeOffsets(
		map[journal.Name]int64{ // DB offsets.
			"journal/part-001": 100,
			"journal/part-002": 200,
			"journal/db-only":  300,
		},
		map[journal.Name]int64{ // Etcd offsets.
			"journal/part-001":  200,
			"journal/part-002":  100,
			"journal/etcd-only": 400,
		}), gc.DeepEquals,
		map[journal.Name]int64{
			"journal/db-only":   300,
			"journal/etcd-only": 400,
			"journal/part-001":  100, // DB is lower than Etcd, but DB wins.
			"journal/part-002":  200,
		})
}

func (s *RoutinesSuite) TestTopicShardMapping(c *gc.C) {
	foo := &topic.Description{Name: "foo", Partitions: 1}
	bar := &topic.Description{Name: "bar", Partitions: 4}
	baz := &topic.Description{Name: "baz", Partitions: 16}

	var topics [3]*topic.Description
	for i, j := range rand.Perm(len(topics)) {
		topics[i] = []*topic.Description{foo, bar, baz}[j]
	}

	group := TopicGroup{Name: "test", Topics: topics[:]}
	n, err := group.NumShards()
	c.Check(n, gc.Equals, 16)
	c.Check(err, gc.IsNil)

	c.Check(group.JournalsForShard(5), gc.DeepEquals,
		map[journal.Name]*topic.Description{
			"foo/part-000": foo, // 5 % 2.
			"bar/part-001": bar, // 5 % 4.
			"baz/part-005": baz, // 5 % 16.
		})
	c.Check(group.JournalsForShard(14), gc.DeepEquals,
		map[journal.Name]*topic.Description{
			"foo/part-000": foo, // 14 % 2.
			"bar/part-002": bar, // 14 % 4.
			"baz/part-014": baz, // 14 % 16.
		})

	// foo => 2 partitions. Expect it's still mappable.
	foo.Partitions = 2
	n, err = group.NumShards()
	c.Check(n, gc.Equals, 16)
	c.Check(err, gc.IsNil)

	c.Check(group.JournalsForShard(7), gc.DeepEquals,
		map[journal.Name]*topic.Description{
			"foo/part-001": foo, // 7 % 2
			"bar/part-003": bar, // 7 % 4
			"baz/part-007": baz, // 7 % 16
		})

	// foo => 3 partitions. Expect it's an invalid configuration.
	foo.Partitions = 3
	_, err = group.NumShards()
	c.Check(err, gc.ErrorMatches, "topic partitions must be multiples of each other")
}

func (s *RoutinesSuite) TestGroupValidation(c *gc.C) {
	// No point starting the consumer if you don't want to consume anything.
	groups := TopicGroups{}
	c.Check(groups.Validate(), gc.ErrorMatches, "must specify at least one TopicGroup")

	// Initially, the two TopicGroups both don't have names.
	groups = TopicGroups{{}, {}}
	c.Check(groups.Validate(), gc.ErrorMatches, "a TopicGroup must have a name")

	// Now assign a special name to the first one.
	groups[0].Name = "Special/Name"
	c.Check(groups.Validate(), gc.ErrorMatches, "a TopicGroup name must consist only of.*")

	// Now the names are both valid, but there's no topics consumed.
	groups[0].Name = "same-name"
	groups[1].Name = "same-name"
	c.Check(groups.Validate(), gc.ErrorMatches, "a TopicGroup must consume at least one topic")

	// Now there are consumed topics, but the names are identical.
	t1 := &topic.Description{Name: "topic-one", Partitions: 3}
	t2 := &topic.Description{Name: "topic-two", Partitions: 4}
	groups[0].Topics = []*topic.Description{t1}
	groups[1].Topics = []*topic.Description{t2}
	c.Check(groups.Validate(), gc.ErrorMatches, "consumer groups must be sorted and names must not repeat: same-name")

	// The names are unique but now groups[0] lexically precedes groups[1].
	groups[0].Name = "the-new-name"
	c.Check(groups.Validate(), gc.ErrorMatches, "consumer groups must be sorted and names must not repeat: same-name")

	// Finally, the groups structure is valid.
	groups[0], groups[1] = groups[1], groups[0]
	c.Check(groups.Validate(), gc.IsNil)
}

func (s *RoutinesSuite) treeFixture() *etcd.Node {
	shard012, _ := json.Marshal(s.hintsFixture())

	return &etcd.Node{
		Key: "/foo", Dir: true,
		Nodes: etcd.Nodes{
			{
				Key: "/foo/hints", Dir: true,
				Nodes: etcd.Nodes{
					{Key: "/foo/hints/shard-bar-030", Value: "... malformed ..."},
					{Key: "/foo/hints/shard-baz-012", Value: string(shard012)},
				},
			}, {
				Key: "/foo/offsets", Dir: true,
				Nodes: etcd.Nodes{
					{
						Key: "/foo/offsets/journal", Dir: true,
						Nodes: etcd.Nodes{
							{Key: "/foo/offsets/journal/part-001", Value: "2a"},
							{Key: "/foo/offsets/journal/part-002", Value: "2b"},
						},
					},
					{
						Key: "/foo/offsets/other-journal", Dir: true,
						Nodes: etcd.Nodes{
							{Key: "/foo/offsets/other-journal/part-002", Value: "2c"},
						},
					},
				},
			},
		},
	}
}

func (s *RoutinesSuite) hintsFixture() recoverylog.FSMHints {
	return recoverylog.FSMHints{
		Log:        "some/recovery/logs/shard-baz-012",
		Properties: []recoverylog.Property{{Path: "foo", Content: "bar"}},
	}
}

var _ = gc.Suite(&RoutinesSuite{})

func Test(t *testing.T) { gc.TestingT(t) }
