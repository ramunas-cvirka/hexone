// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"os"
	"os/user"
	"strconv"
	"sync"
)

// os/user lookups can hit NSS, which is slow enough to matter when a directory
// holds thousands of entries owned by the same few users. Listings run on a
// goroutine, so the cache needs a mutex.
//
// Failed lookups cache their numeric fallback too. That is a deliberate trade,
// not a free win. Within a listing it is the win: a directory full of
// unresolvable uids pays the NSS timeout once instead of once per entry. Across
// a session it is the cost: a transient NSS or LDAP failure pins that uid to
// its numeric form for the life of the process, and an outage is exactly when
// lookups fail, so on an AD-joined machine the only cure is restarting hexone.
// We take that deal because the alternative — retrying every unresolved uid —
// re-pays the timeout on every listing for as long as the outage lasts, and a
// numeric uid is still a correct answer, just an unfriendly one. If the stale
// numeric form ever bites in practice, the fix is a retry policy (a TTL, or
// evicting only the failures), not dropping the cache.
var (
	ownerCacheMu   sync.RWMutex
	userNameCache  = map[uint32]string{}
	groupNameCache = map[uint32]string{}
)

func lookupUserName(uid uint32) string {
	ownerCacheMu.RLock()
	name, ok := userNameCache[uid]
	ownerCacheMu.RUnlock()
	if ok {
		return name
	}

	name = strconv.FormatUint(uint64(uid), 10)
	if resolved, err := user.LookupId(name); err == nil && resolved.Username != "" {
		name = resolved.Username
	}

	ownerCacheMu.Lock()
	userNameCache[uid] = name
	ownerCacheMu.Unlock()
	return name
}

func lookupGroupName(gid uint32) string {
	ownerCacheMu.RLock()
	name, ok := groupNameCache[gid]
	ownerCacheMu.RUnlock()
	if ok {
		return name
	}

	name = strconv.FormatUint(uint64(gid), 10)
	if resolved, err := user.LookupGroupId(name); err == nil && resolved.Name != "" {
		name = resolved.Name
	}

	ownerCacheMu.Lock()
	groupNameCache[gid] = name
	ownerCacheMu.Unlock()
	return name
}

// formatOwnerNames joins the two resolved names. Its only caller passes
// lookupUserName/lookupGroupName results, and those always return at least the
// numeric form, so neither half is ever empty and there is nothing to guard.
func formatOwnerNames(userName, groupName string) string {
	return userName + ":" + groupName
}

func formatOwnerNumeric(uid, gid uint32) string {
	return strconv.FormatUint(uint64(uid), 10) + ":" + strconv.FormatUint(uint64(gid), 10)
}

// localOwnerText resolves uid/gid to names. Only call it for local files: a
// remote uid has no meaning in the local passwd database.
func localOwnerText(info os.FileInfo) string {
	uid, gid, ok := statOwnerIDs(info)
	if !ok {
		return ""
	}
	return formatOwnerNames(lookupUserName(uid), lookupGroupName(gid))
}

// remoteOwnerText keeps uid/gid numeric, which is the only honest rendering for
// an SFTP entry.
func remoteOwnerText(info os.FileInfo) string {
	uid, gid, ok := statOwnerIDs(info)
	if !ok {
		return ""
	}
	return formatOwnerNumeric(uid, gid)
}
