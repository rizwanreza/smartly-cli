import { defineCollection, z } from 'astro:content';
import { glob } from 'astro/loaders';

const docs = defineCollection({
  loader: glob({ pattern: '**/*.mdx', base: './src/content/docs' }),
  schema: z.object({
    title: z.string(),
    description: z.string(),
    /** Short mono label shown above the page title. */
    eyebrow: z.string(),
    /** One-sentence lede under the title. */
    lede: z.string(),
  }),
});

export const collections = { docs };
