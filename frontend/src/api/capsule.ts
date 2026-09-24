import { http } from "@/utils/request";
import type { PageResult } from "./wish";

export interface Capsule {
  id: number;
  user_id: number;
  recipient_id?: number | null;
  owner_username: string;
  owner_nickname: string;
  recipient_username: string;
  recipient_nickname: string;
  title: string;
  content: string;
  image_urls: string[];
  audio_url?: string;
  unlock_at: string;
  status: string;
  unlocked_at?: string | null;
  created_at: string;
  is_owner: boolean;
  can_reply: boolean;
  reply_content: string;
  reply_at?: string | null;
  reply_withdrawn: boolean;
}

export interface CreateCapsulePayload {
  title: string;
  content: string;
  image_urls?: string[];
  audio_url?: string;
  unlock_at: string;
  recipient_username?: string;
}

export const capsuleApi = {
  create: (payload: CreateCapsulePayload) =>
    http.post<Capsule>("/capsules", payload),
  mine: (params?: Record<string, string | number | undefined>) =>
    http.get<PageResult<Capsule>>("/capsules/mine", params),
  received: (params?: Record<string, string | number | undefined>) =>
    http.get<PageResult<Capsule>>("/capsules/received", params),
  detail: (id: number) => http.get<Capsule>(`/capsules/${id}`),
  reply: (id: number, content: string) =>
    http.post<{ reply_at?: string }>(`/capsules/${id}/reply`, { content }),
  withdrawReply: (id: number) =>
    http.del<null>(`/capsules/${id}/reply`),
  remove: (id: number) => http.del<null>(`/capsules/${id}`),
};
