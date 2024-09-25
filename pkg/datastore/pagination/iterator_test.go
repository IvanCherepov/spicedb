package pagination

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/authzed/spicedb/internal/datastore/common"
	"github.com/authzed/spicedb/pkg/datastore"
	"github.com/authzed/spicedb/pkg/datastore/options"
	core "github.com/authzed/spicedb/pkg/proto/core/v1"
	"github.com/authzed/spicedb/pkg/tuple"
)

func TestPaginatedIterator(t *testing.T) {
	testCases := []struct {
		order              options.SortOrder
		pageSize           uint64
		totalRelationships uint64
	}{
		{options.ByResource, 1, 0},
		{options.ByResource, 1, 1},
		{options.ByResource, 1, 10},
		{options.ByResource, 10, 10},
		{options.ByResource, 100, 10},
		{options.ByResource, 10, 1000},
		{options.ByResource, 9, 20},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d/%d-%d", tc.pageSize, tc.totalRelationships, tc.order), func(t *testing.T) {
			t.Parallel()
			require := require.New(t)

			rels := make([]tuple.Relationship, 0, tc.totalRelationships)
			for i := 0; i < int(tc.totalRelationships); i++ {
				rels = append(rels, tuple.Relationship{
					RelationshipReference: tuple.RelationshipReference{
						Resource: tuple.ObjectAndRelation{
							ObjectType: "document",
							ObjectID:   strconv.FormatUint(i, 10),
							Relation:   "owner",
						},
						Subject: tuple.ObjectAndRelation{
							ObjectType: "user",
							ObjectID:   strconv.FormatUint(i, 10),
							Relation:   datastore.Ellipsis,
						},
					},
				})
			}

			ds := generateMock(rels, tc.pageSize, options.ByResource)

			ctx := context.Background()
			iter, err := NewPaginatedIterator(ctx, ds, datastore.RelationshipsFilter{
				OptionalResourceType: "unused",
			}, tc.pageSize, options.ByResource, nil)
			require.NoError(err)

			slice, err := datastore.IteratorToSlice(iter)
			require.NoError(err)
			require.Len(slice, int(tc.totalRelationships))

			// Make sure everything got called
			require.True(ds.AssertExpectations(t))
		})
	}
}

func generateMock(rels []tuple.Relationship, pageSize uint64, order options.SortOrder) *mockedReader {
	mock := &mockedReader{}
	relsLen := uint64(len(rels))

	var last options.Cursor
	for i := uint64(0); i <= relsLen; i += pageSize {
		pastLastIndex := i + pageSize
		if pastLastIndex > relsLen {
			pastLastIndex = relsLen
		}

		iter := common.NewSliceRelationshipIterator(rels[i:pastLastIndex], order)
		mock.On("QueryRelationships", last, order, pageSize).Return(iter, nil)
		if relsLen > 0 {
			l := rels[pastLastIndex-1]
			last = options.Cursor(&l)
		}
	}

	return mock
}

type mockedReader struct {
	mock.Mock
}

var _ datastore.Reader = &mockedReader{}

func (m *mockedReader) QueryRelationships(
	_ context.Context,
	_ datastore.RelationshipsFilter,
	opts ...options.QueryOptionsOption,
) (datastore.RelationshipIterator, error) {
	queryOpts := options.NewQueryOptionsWithOptions(opts...)
	args := m.Called(queryOpts.After, queryOpts.Sort, *queryOpts.Limit)
	potentialRelIter := args.Get(0)
	if potentialRelIter == nil {
		return nil, args.Error(1)
	}
	return potentialRelIter.(datastore.RelationshipIterator), args.Error(1)
}

func (m *mockedReader) ReverseQueryRelationships(
	_ context.Context,
	_ datastore.SubjectsFilter,
	_ ...options.ReverseQueryOptionsOption,
) (datastore.RelationshipIterator, error) {
	panic("not implemented")
}

func (m *mockedReader) CountRelationships(ctx context.Context, name string) (int, error) {
	panic("not implemented")
}

func (m *mockedReader) LookupCounters(ctx context.Context) ([]datastore.RelationshipCounter, error) {
	panic("not implemented")
}

func (m *mockedReader) ReadCaveatByName(_ context.Context, _ string) (caveat *core.CaveatDefinition, lastWritten datastore.Revision, err error) {
	panic("not implemented")
}

func (m *mockedReader) ListAllCaveats(_ context.Context) ([]datastore.RevisionedCaveat, error) {
	panic("not implemented")
}

func (m *mockedReader) LookupCaveatsWithNames(_ context.Context, _ []string) ([]datastore.RevisionedCaveat, error) {
	panic("not implemented")
}

func (m *mockedReader) ReadNamespaceByName(_ context.Context, _ string) (ns *core.NamespaceDefinition, lastWritten datastore.Revision, err error) {
	panic("not implemented")
}

func (m *mockedReader) ListAllNamespaces(_ context.Context) ([]datastore.RevisionedNamespace, error) {
	panic("not implemented")
}

func (m *mockedReader) LookupNamespacesWithNames(_ context.Context, _ []string) ([]datastore.RevisionedNamespace, error) {
	panic("not implemented")
}

type mockedIterator struct {
	mock.Mock
}
