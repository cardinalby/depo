import React from 'react';

interface ErrorBoundaryState {
    hasError: boolean;
    error?: Error;
}

class ErrorBoundary extends React.Component<React.PropsWithChildren<{}>, ErrorBoundaryState> {
    constructor(props: {}) {
        super(props);
        this.state = {hasError: false};
    }

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return {hasError: true, error};
    }

    componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
        // eslint-disable-next-line no-console
        console.error('ErrorBoundary caught an error:', error, errorInfo);
    }

    render() {
        if (this.state.hasError) {
            return (
                <div className="error-message">
                    <div>Something went wrong.</div>
                    {this.state.error && <div style={{marginTop: 8, fontSize: 12}}>{this.state.error.message}</div>}
                </div>
            );
        }

        return this.props.children;
    }
}

export default ErrorBoundary;
