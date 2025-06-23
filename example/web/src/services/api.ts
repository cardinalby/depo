import {Graph, ResetRequest, StopComponentRequest, UpdateComponentRequest} from '../types/api';
import {callWasmMethod} from "./wasmHandler";

declare var isWasmHandlerReady: Promise<void>

class ApiService {
    ready(): Promise<void> {
        return isWasmHandlerReady
    }

    fetchGraph(): Graph {
        return callWasmMethod("graph", null)
    }

    startSystem(){
        callWasmMethod("startAll", null)
    }

    async stopSystem() {
        callWasmMethod("stopAll", null)
    }

    resetSystem(request: ResetRequest) {
        callWasmMethod("reset", request)
    }

    updateComponent(request: UpdateComponentRequest) {
        callWasmMethod("updateComponent", request)
    }

    stopComponent(request: StopComponentRequest) {
        callWasmMethod("stopComponent", request)
    }
}

export const apiService = new ApiService();
