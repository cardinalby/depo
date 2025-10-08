import React, {createContext, useCallback, useContext, useState} from 'react';
import {useGraph} from './hooks/useGraph';
import {Component} from './types/api';
import {apiService} from './services/api';
import DependencyGraph from './components/DependencyGraph';
import Toolbar from './components/Toolbar';
import ComponentModal from './components/ComponentModal';
import StatusIndicator from './components/StatusIndicator';
import Legend from './components/Legend';
import ErrorBoundary from './components/ErrorBoundary';

interface AppContextType {
    selectedComponent: Component | null;
    setSelectedComponent: (component: Component | null) => void;
    isSystemIdle: boolean;
}

const AppContext = createContext<AppContextType | null>(null);

export const useAppContext = () => {
    const context = useContext(AppContext);
    if (!context) {
        throw new Error('useAppContext must be used within AppProvider');
    }
    return context;
};

const App: React.FC = () => {
    const {graph, loading, error, refetch} = useGraph();
    const [selectedComponent, setSelectedComponent] = useState<Component | null>(null);

    const isSystemIdle = graph?.status === 'pending' || graph?.status === 'non_runnable';

    const shutDownOnNilRunResult = !!graph?.shut_down_on_nil_run_result;

    const handleStartSystem = () => {
        try {
            apiService.startSystem();
            refetch()
        } catch (err) {
            console.error('Failed to start system:', err);
        }
    };

    const handleStopSystem = () => {
        try {
            apiService.stopSystem();
            refetch()
        } catch (err) {
            console.error('Failed to stop system:', err);
        }
    };

    const handleResetSystem = (shut_down_on_nil_run_result: boolean) => {
        try {
            apiService.resetSystem({shut_down_on_nil_run_result});
            refetch()
        } catch (err) {
            console.error('Failed to reset system:', err);
        }
    };

    const handleNodeClick = useCallback((component: Component) => {
        setSelectedComponent(component);
    }, []);

    const contextValue: AppContextType = {
        selectedComponent,
        setSelectedComponent,
        isSystemIdle,
    };

    if (loading) {
        return (
            <div className="app">
                <div className="loading-message">Loading...</div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="app">
                <div className="error-message">Error: {error}</div>
            </div>
        );
    }

    if (!graph) {
        return (
            <div className="app">
                <div className="error-message">No graph data available</div>
            </div>
        );
    }

    return (
        <AppContext.Provider value={contextValue}>
            <ErrorBoundary>
                <div className="app">
                    <Toolbar
                        isIdle={isSystemIdle}
                        shutDownOnNilRunResult={shutDownOnNilRunResult}
                        onStart={handleStartSystem}
                        onStop={handleStopSystem}
                        onReset={handleResetSystem}
                    />

                    <StatusIndicator status={graph.status} runnerError={graph.runner_error}/>

                    <div className="graph-container">
                        <DependencyGraph
                            components={graph.components}
                            onNodeClick={handleNodeClick}
                        />
                    </div>

                    <Legend/>

                    {selectedComponent && (
                        <ComponentModal
                            component={selectedComponent}
                            onClose={() => setSelectedComponent(null)}
                            onUpdate={refetch}
                        />
                    )}
                </div>
            </ErrorBoundary>
        </AppContext.Provider>
    );
};

export default App;
