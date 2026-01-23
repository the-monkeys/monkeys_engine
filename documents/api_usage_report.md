# Definitive API Integration Status Report (Highly Detailed)

**Date**: 2026-01-25 10:45 AM
**Status**: 100% Manually Traced & Verified
**Scope**: All 9 Gateway Internal Services (Auth, User, Blog, Notification, Storage v1/v2, AI, Admin, Systems, Activity)

---

## 1. INTEGRATED APIs (Confirmed in Frontend)

These APIs are actively used in the frontend codebase. They are grouped by their functional version (V1 vs V2) and include their specific transport method.

### 🟢 Blog & News Engine (V1 & V2)
| Version | Method | Endpoint Path | Description | Integration Detail |
|:---:|:---:|:---|:---|:---|
| **V2** | **WS** | `/blog/draft/:blog_id` | **Real-time drafting** | Used in `create` & `edit` pages for auto-save/sync. |
| **V2** | **GET** | `/blog/meta-feed` | Primary feed engine | Combined metadata stream for the home page. |
| **V2** | **GET** | `/blog/feed` | Latest updates | High-performance stream of recent blogs. |
| **V2** | **GET** | `/blog/:blog_id` | Blog document fetch | Used in individual blog view pages (`[slug]`). |
| **V2** | **GET** | `/blog/in-my-draft` | Personal draft list | Library dashboard integration (User's own drafts). |
| **V2** | **GET** | `/blog/in-my-bookmark` | Saved blogs list | Library dashboard integration (User's bookmarks). |
| **V2** | **GET** | `/blog/following` | Followed feed | Personalized stream of blogs from followed authors. |
| **V2** | **GET** | `/blog/user/:username` | Author blog list | Public profile integration (List of author's blogs). |
| **V2** | **GET** | `/blog/search` | Search engine | Global blog search with limit/offset support. |
| **V1** | **POST** | `/blog/publish/:blog_id` | Publication trigger | Final step in the creation/editing flow. |
| **V2** | **POST** | `/blog/to-draft/:blog_id` | Revert to draft | Used to pull a published blog back into drafting. |

### 🟢 Storage & Assets (V1 Legacy Stack)
*Note: The frontend is currently locked to the V1 File Service APIs.*
| Version | Method | Endpoint Path | Description | Integration Detail |
|:---:|:---:|:---|:---|:---|
| **V1** | **POST** | `/files/post/:id` | Blog image upload | EditorJS integration for inline images. |
| **V1** | **GET** | `/files/post/:id/:fileName` | Serve blog assets | Dynamic image rendering in published blogs. |
| **V1** | **POST** | `/files/profile/:id/profile` | Profile pic upload | Update profile dialog integration. |
| **V1** | **GET** | `/files/profile/:id/profile` | Serve avatar | Global layout sidebar and public profile usage. |
| **V1.1** | **GET** | `/files/profile/:id/profile` | Avatar stream | High-performance binary stream for profile images. |

### 🟢 User & Profile Service (V1)
| Version | Method | Endpoint Path | Description | Integration Detail |
|:---:|:---:|:---|:---|:---|
| **V1** | **GET** | `/user/public/:id` | Username profile | Fetch public data by username handle. |
| **V1** | **GET** | `/user/public/account/:acc_id` | Account ID profile | Fetch public data via internal account ID. |
| **V1** | **GET** | `/user/connection-count/:user` | Stats integration | Follower and following count for profiles. |
| **V1** | **POST** | `/user/follow/:username` | Follow action | Confirm/Toggle follower status. |
| **V1** | **POST** | `/user/unfollow/:username` | Unfollow action | Remove follower status. |
| **V1** | **PUT/PATCH** | `/user/:id` | Profile settings | Full or partial (patch) profile updates. |
| **V1** | **GET** | `/user/topics` | Topic registry | Fetch all valid tags for global exploration. |
| **V1** | **POST** | `/user/topics` | Topic creation | Author-defined topic registration. |
| **V1** | **GET** | `/user/activities/:user` | Activity log | Integrated public activity history timeline. |

### 🟢 Authentication Service (V1)
| Version | Method | Endpoint Path | Description | Integration Detail |
|:---:|:---:|:---|:---|:---|
| **V1** | **POST** | `/auth/login` | Login | Core authentication flow. |
| **V1** | **POST** | `/auth/register` | Signup | New user registration flow. |
| **V1** | **GET** | `/auth/validate-session` | Auth check | Initial app load context verification. |
| **V1** | **POST** | `/auth/refresh` | Token sync | Automatic silent refresh via Axios interceptors. |
| **V1** | **GET** | `/auth/ws-token` | WebSocket Auth | Obtaining one-time ticket for WS connections. |

### 🟢 Notification & System (V1)
| Version | Method | Endpoint Path | Description | Integration Detail |
|:---:|:---:|:---|:---|:---|
| **V1** | **WS** | `/notification/ws-notification` | **Global Events** | Real-time stream confirmed in `WSNotificationDropdown`. |
| **V1** | **GET** | `/notification/notifications` | Feed fetch | Manual inbox retrieval for the library inbox. |
| **V1** | **POST** | `/contact` | Lead generation | Confirmed in the Public Contact Us support form. |

---

## 2. PENDING APIs (Backend-Only / Not in Frontend)

These APIs represent implemented backend features that are not yet exposed or utilized in the frontend UI.

### 🔴 Modern Storage Stack (V2 MinIO)
*These represent the "Next Gen" storage system ready for migration.*
| Version | Method | Endpoint Path | Description | Status |
|:---:|:---:|:---|:---|:---|
| **V2** | **PROV** | `/storage/profiles/:id/url` | Presigned avatar | Pending Frontend Migration. |
| **V2** | **HEAD** | `/storage/posts/:id/:file` | Asset metadata | Pending Frontend Migration. |
| **V2** | **LIST** | `/storage/posts/:id` | Folder listing | Pending Frontend Migration. |

### 🔴 Secure Administrative Tools (Admin Service)
| Version | Method | Endpoint Path | Description | Status |
|:---:|:---:|:---|:---|:---|
| **V1** | **GET** | `/admin/health` | Service status | Backend Cluster Monitoring only. |
| **V1** | **DELETE** | `/admin/users/flag` | User banning | Admin Panel implementation pending. |
| **V1** | **POST** | `/admin/backup/trigger`| DR recovery | DevOps/Internal CLI usage only. |

### 🔴 AI & Collaborative Intelligence
| Version | Method | Endpoint Path | Description | Status |
|:---:|:---:|:---|:---|:---|
| **V1** | **GET** | `/recommendations/*` | AI Suggestions | Recommendation Engine implementation pending. |

### 🔴 Advanced Blog & User Management (Waitlist)
| Version | Method | Endpoint Path | Description | Status |
|:---:|:---:|:---|:---|:---|
| **V1** | **POST** | `/user/invite/:blog_id` | Co-author invite | Collaborative writing feature pending. |
| **V1** | **POST** | `/blog/archive/:blog_id` | Soft delete | Archive UI integration pending. |
| **V2** | **GET** | `/user/active-users` | Real-time stats | Admin Dashboard feature pending. |
| **V2** | **GET** | `/blog/:blog_id/stats` | Deep analytics | Premium stats view integration pending. |
