declare function wasmHandler(req: string): string;

export function callWasmMethod(
    methodName: string,
    body: any,
): any {
    const req = JSON.stringify({
        method: methodName,
        body: body,
    })
    const response = wasmHandler(req);
    const parsed = JSON.parse(response)
    if (parsed.error) {
        throw new Error(parsed.error);
    }
    return parsed.body
}