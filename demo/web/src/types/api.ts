export type ComponentStatus =
    | 'non_runnable'
    | 'pending'
    | 'starting'
    | 'ready'
    | 'closing'
    | 'done';

export interface Component {
    id: number;
    name: string;
    depends_on?: number[];
    start_error?: string;
    delay: number;
    status: ComponentStatus;
    done_error?: string;
}

export interface Graph {
    components: Component[];
    status: ComponentStatus;
    // can contain non-empty string if status is 'done' indicating the run result
    runner_error: string;
    shut_down_on_nil_run_result: boolean
}

export interface UpdateComponentRequest {
    component_id: number;
    delay_ms: number;
    start_error: string;
}

export interface StopComponentRequest {
    component_id: number;
    with_error: boolean;
}

export interface ResetRequest {
    shut_down_on_nil_run_result: boolean;
}
