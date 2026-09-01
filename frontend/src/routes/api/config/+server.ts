import { json } from '@sveltejs/kit';
import { config } from '$lib/server/config';

export function GET() {
	return json({ defaultBaseURL: config.defaultBaseURL });
}
