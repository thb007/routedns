package rdns

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/miekg/dns"
)

// Replace is a resolver that modifies queries according to regular expressions
// and forwards the modified queries to another resolver. Responses are then
// mapped back to the original query string.
type Replace struct {
	resolver Resolver
	exp      replaceExpressions
}

var _ Resolver = &Replace{}

type replaceExp struct {
	from *regexp.Regexp
	to   string
}

type replaceExpressions []replaceExp

func (r replaceExpressions) apply(name string) string {
	for _, e := range r {
		name = e.from.ReplaceAllString(name, e.to)
	}
	return name
}

type ReplaceOperation struct {
	From string
	To   string
}

// NewReplace returns a new instance of a Replace resolver.
func NewReplace(resolver Resolver, list ...ReplaceOperation) (*Replace, error) {
	var exp replaceExpressions
	for _, o := range list {
		re, err := regexp.Compile(o.From)
		if err != nil {
			return nil, err
		}
		exp = append(exp, replaceExp{re, o.To})
	}

	return &Replace{resolver: resolver, exp: exp}, nil
}

// Resolve a DNS query by first replacing the query string with another
// sending the query upstream and replace the name in the response with
// the orignal query string again.
func (r *Replace) Resolve(q *dns.Msg, ci ClientInfo) (*dns.Msg, error) {
	if len(q.Question) < 1 {
		return nil, errors.New("no question in query")
	}
	oldName := q.Question[0].Name
	newName := r.exp.apply(oldName)

	// if nothing needs modifying, we can stop here and use the original query
	if newName == oldName {
		return r.resolver.Resolve(q, ci)
	}

	// Modify the query string
	q.Question[0].Name = newName

	// Send the query upstream
	a, err := r.resolver.Resolve(q, ci)
	if err != nil {
		return nil, err
	}

	// Set the question back to the original name
	a.Question[0].Name = oldName

	// Now put the original name in all answer records that have the
	// new name
	for _, answer := range a.Answer {
		if answer.Header().Name == newName {
			answer.Header().Name = oldName
		}
	}
	return a, nil
}

func (r *Replace) String() string {
	return fmt.Sprintf("Replace(resolver=%s)", r.resolver)
}
