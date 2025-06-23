import {useState, useEffect, useCallback, useRef} from 'react';
import {Graph} from '../types/api';
import {apiService} from '../services/api';

export const useGraph = () => {
    const [graph, setGraph] = useState<Graph | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const intervalRef = useRef<NodeJS.Timeout | null>(null);

    const fetchGraph = useCallback((force = false) => {
        try {
            setError(null);
            const newGraph = apiService.fetchGraph();
            setGraph(newGraph)
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Unknown error');
        }
    }, []);

    const startPolling = useCallback(() => {
        if (intervalRef.current) return;

        intervalRef.current = setInterval(() => {
            fetchGraph(false);
        }, 200);
    }, [fetchGraph]);

    const stopPolling = useCallback(() => {
        if (intervalRef.current) {
            clearInterval(intervalRef.current);
            intervalRef.current = null;
        }
    }, []);

    useEffect(() => {
        (async () => {
            setLoading(true)
            await apiService.ready()
            fetchGraph(true)
            setLoading(false)
            startPolling()
        })()
        return () => stopPolling();
    }, [fetchGraph, startPolling, stopPolling]);

    return {
        graph,
        loading,
        error,
        refetch: () => fetchGraph(true),
    };
};
