import sharp from 'sharp'

const [format, qualityValue, losslessValue] = process.argv.slice(-3)
const quality = Number(qualityValue)
const lossless = losslessValue === 'true'

if (!['webp', 'avif'].includes(format)) throw new Error('Format must be webp or avif')
if (!Number.isInteger(quality) || quality < 1 || quality > 100) {
  throw new Error('Quality must be an integer from 1 to 100')
}

const chunks = []
for await (const chunk of process.stdin) chunks.push(chunk)

let encoder = sharp(Buffer.concat(chunks), { failOn: 'error' })
if (format === 'webp') {
  encoder = encoder.webp({
    quality,
    lossless,
    alphaQuality: 100,
    effort: 6,
    smartSubsample: true,
    smartDeblock: true,
    preset: 'picture',
    exact: true,
  })
} else {
  encoder = encoder.avif({
    quality,
    lossless,
    effort: 9,
    chromaSubsampling: '4:4:4',
    bitdepth: 8,
    tune: 'ssim',
  })
}

process.stdout.write(await encoder.toBuffer())
