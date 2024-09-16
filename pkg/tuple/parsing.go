package tuple

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"google.golang.org/protobuf/types/known/structpb"

	core "github.com/authzed/spicedb/pkg/proto/core/v1"
)

const (
	namespaceNameExpr = "([a-z][a-z0-9_]{1,61}[a-z0-9]/)*[a-z][a-z0-9_]{1,62}[a-z0-9]"
	resourceIDExpr    = "([a-zA-Z0-9/_|\\-=+]{1,})"
	subjectIDExpr     = "([a-zA-Z0-9/_|\\-=+]{1,})|\\*"
	relationExpr      = "[a-z][a-z0-9_]{1,62}[a-z0-9]"
	caveatNameExpr    = "([a-z][a-z0-9_]{1,61}[a-z0-9]/)*[a-z][a-z0-9_]{1,62}[a-z0-9]"
)

var onrExpr = fmt.Sprintf(
	`(?P<resourceType>(%s)):(?P<resourceID>%s)#(?P<resourceRel>%s)`,
	namespaceNameExpr,
	resourceIDExpr,
	relationExpr,
)

var subjectExpr = fmt.Sprintf(
	`(?P<subjectType>(%s)):(?P<subjectID>%s)(#(?P<subjectRel>%s|\.\.\.))?`,
	namespaceNameExpr,
	subjectIDExpr,
	relationExpr,
)

var caveatExpr = fmt.Sprintf(`\[(?P<caveatName>(%s))(:(?P<caveatContext>(\{(.+)\})))?\]`, caveatNameExpr)

var (
	onrRegex        = regexp.MustCompile(fmt.Sprintf("^%s$", onrExpr))
	subjectRegex    = regexp.MustCompile(fmt.Sprintf("^%s$", subjectExpr))
	resourceIDRegex = regexp.MustCompile(fmt.Sprintf("^%s$", resourceIDExpr))
	subjectIDRegex  = regexp.MustCompile(fmt.Sprintf("^%s$", subjectIDExpr))
)

var parserRegex = regexp.MustCompile(
	fmt.Sprintf(
		`^%s@%s(%s)?$`,
		onrExpr,
		subjectExpr,
		caveatExpr,
	),
)

// ValidateResourceID ensures that the given resource ID is valid. Returns an error if not.
func ValidateResourceID(objectID string) error {
	if !resourceIDRegex.MatchString(objectID) {
		return fmt.Errorf("invalid resource id; must match %s", resourceIDExpr)
	}
	if len(objectID) > 1024 {
		return fmt.Errorf("invalid resource id; must be <= 1024 characters")
	}

	return nil
}

// ValidateSubjectID ensures that the given object ID (under a subject reference) is valid. Returns an error if not.
func ValidateSubjectID(subjectID string) error {
	if !subjectIDRegex.MatchString(subjectID) {
		return fmt.Errorf("invalid subject id; must be alphanumeric and between 1 and 127 characters or a star for public")
	}
	if len(subjectID) > 1024 {
		return fmt.Errorf("invalid resource id; must be <= 1024 characters")
	}

	return nil
}

// MustParse wraps Parse such that any failures panic rather than returning an error.
func MustParse(relString string) Relationship {
	parsed, err := Parse(relString)
	if err != nil {
		panic(err)
	}
	return parsed
}

// Parse unmarshals the string form of a Tuple and returns an error on failure,
//
// This function treats both missing and Ellipsis relations equally.
func Parse(relString string) (Relationship, error) {
	groups := parserRegex.FindStringSubmatch(relString)
	if len(groups) == 0 {
		return Relationship{}, fmt.Errorf("invalid relationship string")
	}

	subjectRelation := Ellipsis
	subjectRelIndex := slices.Index(parserRegex.SubexpNames(), "subjectRel")
	if len(groups[subjectRelIndex]) > 0 {
		subjectRelation = groups[subjectRelIndex]
	}

	caveatName := groups[slices.Index(parserRegex.SubexpNames(), "caveatName")]
	var optionalCaveat *core.ContextualizedCaveat
	if caveatName != "" {
		optionalCaveat = &core.ContextualizedCaveat{
			CaveatName: caveatName,
		}

		caveatContextString := groups[slices.Index(parserRegex.SubexpNames(), "caveatContext")]
		if len(caveatContextString) > 0 {
			contextMap := make(map[string]any, 1)
			err := json.Unmarshal([]byte(caveatContextString), &contextMap)
			if err != nil {
				return Relationship{}, fmt.Errorf("invalid caveat context JSON: %w", err)
			}

			caveatContext, err := structpb.NewStruct(contextMap)
			if err != nil {
				return Relationship{}, fmt.Errorf("invalid caveat context: %w", err)
			}

			optionalCaveat.Context = caveatContext
		}
	}

	resourceID := groups[slices.Index(parserRegex.SubexpNames(), "resourceID")]
	if err := ValidateResourceID(resourceID); err != nil {
		return Relationship{}, fmt.Errorf("invalid resource id: %w", err)
	}

	subjectID := groups[slices.Index(parserRegex.SubexpNames(), "subjectID")]
	if err := ValidateSubjectID(subjectID); err != nil {
		return Relationship{}, fmt.Errorf("invalid subject id: %w", err)
	}

	return Relationship{
		RelationshipReference: RelationshipReference{
			Resource: ObjectAndRelation{
				ObjectType: groups[slices.Index(parserRegex.SubexpNames(), "resourceType")],
				ObjectID:   resourceID,
				Relation:   groups[slices.Index(parserRegex.SubexpNames(), "resourceRel")],
			},
			Subject: ObjectAndRelation{
				ObjectType: groups[slices.Index(parserRegex.SubexpNames(), "subjectType")],
				ObjectID:   subjectID,
				Relation:   subjectRelation,
			},
		},
		OptionalCaveat: optionalCaveat,
	}, nil
}

// MustWithCaveat adds the given caveat name to the relationship. This is for testing only.
func MustWithCaveat(rel Relationship, caveatName string, contexts ...map[string]any) Relationship {
	wc, err := WithCaveat(rel, caveatName, contexts...)
	if err != nil {
		panic(err)
	}
	return wc
}

// WithCaveat adds the given caveat name to the relationship. This is for testing only.
func WithCaveat(rel Relationship, caveatName string, contexts ...map[string]any) (Relationship, error) {
	var context *structpb.Struct

	if len(contexts) > 0 {
		combined := map[string]any{}
		for _, current := range contexts {
			maps.Copy(combined, current)
		}

		contextStruct, err := structpb.NewStruct(combined)
		if err != nil {
			return Relationship{}, err
		}
		context = contextStruct
	}

	rel.OptionalCaveat = &core.ContextualizedCaveat{
		CaveatName: caveatName,
		Context:    context,
	}
	return rel, nil
}

// StringToONR creates an ONR from string pieces.
func StringToONR(ns, oid, rel string) ObjectAndRelation {
	return ObjectAndRelation{
		ObjectType: ns,
		ObjectID:   oid,
		Relation:   rel,
	}
}
