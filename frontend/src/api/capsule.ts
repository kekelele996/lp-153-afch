import { http } from "@/utils/request";
import type { PageResult } from "./wish";

export interface CapsuleReply {
  id: number;
  capsule_id: number;
  user_id: number;
  content: string;
  status: string; // active / withdrawn
  withdrawn_at?: string | null;
  created_at: string;
}

export interface Capsule {
  id: number;
  user_id: number;
  owner_username: string;
  owner_nickname: string;
  title: string;
  content: string;
  image_urls: string[];
  audio_url?: string;
  unlock_at: string;
  status: string;
  unlocked_at?: string | null;
  created_at: string;
  recipient_id?: number | null;
  recipient_username: string;
  role: "owner" | "recipient";
  reply?: CapsuleReply | null;
}

export const capsuleApi = {
  create: (payload: {
    title: string;
    content: string;
    image_urls?: string[];
    audio_url?: string;
    unlock_at: string;
    recipient_username?: string;
  }) => http.post<Capsule>("/capsules", payload),
  mine: (params?: Record<string, string | number | undefined>) =>
    http.get<PageResult<Capsule>>("/capsules/mine", params),
  received: (params?: Record<string, string | number | undefined>) =>
    http.get<PageResult<Capsule>>("/capsules/received", params),
  detail: (id: number) => http.get<Capsule>(`/capsules/${id}`),
  remove: (id: number) => http.del<null>(`/capsules/${id}`),
  reply: (id: number, content: string) =>
    http.post<CapsuleReply>(`/capsules/${id}/reply`, { content }),
  withdrawReply: (id: number) => http.del<null>(`/capsules/${id}/reply`),
};
