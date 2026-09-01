import QRCode from 'qrcode';

// Generate a QR code data URL for a text value, entirely in the browser.
export async function qrDataUrl(text: string): Promise<string> {
	return QRCode.toDataURL(text, {
		errorCorrectionLevel: 'M',
		margin: 2,
		width: 1024
	});
}

// Trigger a client-side download of a data URL.
export function downloadDataUrl(dataUrl: string, filename: string): void {
	const a = document.createElement('a');
	a.href = dataUrl;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	a.remove();
}
