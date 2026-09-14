export interface HealthResponse {
  status: string;
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    request_id?: string;
    details?: { field: string; message: string }[];
  };
}

export type ServiceState = "checking" | "up" | "down";

export interface ServiceStatus {
  name: string;
  endpoint: string;
  state: ServiceState;
  detail: string;
  checkedAt: Date | null;
}
