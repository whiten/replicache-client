package db

import (
	"fmt"

	"github.com/attic-labs/noms/go/marshal"
	"github.com/attic-labs/noms/go/types"
	"github.com/attic-labs/noms/go/util/datetime"

	"github.com/aboodman/replicant/util/noms/reachable"
)

// rebase transforms a forked commit history into a linear one by moving one side
// of the fork such that it comes after the other side.
// Specifically rebase finds the forkpoint between `commit` and `onto`. The commits
// after this forkpoint on the `commit` side are replayed one by one on top of onto,
// and the resulting new head is returned.
//
// In Replicant, unlike e.g., Git, this is done such that the original forked
// history is still preserved in the database (e.g. for later debugging). But the
// effect on the data and from user's point of view is the same as `git rebase`.
func rebase(db *DB, reachable *reachable.Set, onto types.Ref, date datetime.DateTime, commit Commit) (rebased Commit, err error) {
	// If `commit` is reachable from `onto`, then we've found our fork point.
	// Thus, by definition, `onto` is the result.
	if reachable.Has(commit.Original.Hash()) {
		var r Commit
		err = marshal.Unmarshal(onto.TargetValue(db.noms), &r)
		if err != nil {
			return Commit{}, err
		}
		return r, nil
	}

	// Otherwise, we recurse on this commit's basis.
	oldBasis, err := commit.Basis(db.noms)
	if err != nil {
		return Commit{}, err
	}
	newBasis, err := rebase(db, reachable, onto, date, oldBasis)
	if err != nil {
		return Commit{}, err
	}

	// If the current and desired basis match, this is a fast-forward, and there's nothing to do.
	if newBasis.Original.Equals(oldBasis.Original) {
		return commit, nil
	}

	// Otherwise we need to re-execute the transaction against the new basis.
	var newBundle, newData types.Ref

	switch commit.Type() {
	case CommitTypeTx:
		// For Tx transactions, just re-run the tx with the new basis.
		newBundle, newData, _, _, err = db.execImpl(types.NewRef(newBasis.Original), commit.Meta.Tx.Bundle(db.noms), commit.Meta.Tx.Name, commit.Meta.Tx.Args)
		if err != nil {
			return Commit{}, err
		}
		break

	case CommitTypeReorder:
		// Reorder transactions can be recursive. But at the end of the chain there will eventually be an original Tx function.
		// Find it and re-run it against the new basis.
		target, err := commit.FinalReorderTarget(db.noms)
		if err != nil {
			return Commit{}, err
		}
		newBundle, newData, _, _, err = db.execImpl(types.NewRef(newBasis.Original), target.Meta.Tx.Bundle(db.noms), target.Meta.Tx.Name, target.Meta.Tx.Args)
		if err != nil {
			return Commit{}, err
		}

	default:
		return Commit{}, fmt.Errorf("Cannot rebase commit of type %s: %s: %s", commit.Type(), commit.Original.Hash(), types.EncodedValue(commit.Original))
	}

	// Create and return the reorder commit, which will become the basis for the prev frame of the recursive call.
	newCommit := makeReorder(db.noms, types.NewRef(newBasis.Original), db.origin, date, types.NewRef(commit.Original), newBundle, newData)
	db.noms.WriteValue(newCommit.Original)
	return newCommit, nil
}
