// Экспорт тренировки в PDF через pdfmake.
// Библиотека (вместе со шрифтами — более 1 МБ) импортируется динамически,
// чтобы не раздувать основной бандл: код загрузится только по клику на кнопку.
// Шрифт Roboto с поддержкой кириллицы встроен в pdfmake по умолчанию.

// Набор колонок таблицы подходов зависит от типа записи упражнения каталога
function columnsFor(trackingType) {
  switch (trackingType) {
    case 'weight_reps':
      return ['weight_kg', 'reps', 'rest_sec']
    case 'reps':
      return ['reps', 'rest_sec']
    case 'time':
      return ['duration_sec', 'rest_sec']
    case 'distance_time':
      return ['distance_m', 'duration_sec', 'rest_sec']
    default:
      return ['reps', 'rest_sec']
  }
}

function formatValue(set, field, t) {
  const value = set[field]
  if (value == null) return '—'
  if (field === 'rest_sec') return t('programs.restSeconds', value)
  if (field === 'duration_sec') return t('programs.durationSeconds', value)
  return String(value)
}

const MUTED_COLOR = '#64748b'
const IMAGE_SIZE = 110

function blobToDataUri(blob) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result)
    reader.onerror = () => reject(new Error('Failed to read media blob'))
    reader.readAsDataURL(blob)
  })
}

// GIF/WebP pdfmake не поддерживает — рисуем первый кадр на canvas и отдаём PNG
function imageToPngDataUri(dataUri) {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      const canvas = document.createElement('canvas')
      canvas.width = img.naturalWidth
      canvas.height = img.naturalHeight
      canvas.getContext('2d').drawImage(img, 0, 0)
      resolve(canvas.toDataURL('image/png'))
    }
    img.onerror = () => reject(new Error('Failed to decode image'))
    img.src = dataUri
  })
}

// Возвращает data URI картинки или null, если загрузить не удалось
// (блок упражнения тогда остаётся текстовым — как при ошибке <img> в UI)
async function loadImageDataUri(url) {
  try {
    const response = await fetch(url)
    if (!response.ok) return null
    const blob = await response.blob()
    const dataUri = await blobToDataUri(blob)
    if (blob.type === 'image/jpeg' || blob.type === 'image/png') return dataUri
    return await imageToPngDataUri(dataUri)
  } catch {
    return null
  }
}

// Для PDF предпочитаем статичное JPEG-превью каталога, GIF — запасной вариант
function mediaUrl(exercise) {
  return exercise.thumbnail_url || exercise.gif_url || ''
}

async function loadImages(detail) {
  const urls = [
    ...new Set(
      (detail.days ?? [])
        .flatMap((day) => day.exercises)
        .map(mediaUrl)
        .filter(Boolean),
    ),
  ]
  const entries = await Promise.all(
    urls.map(async (url) => [url, await loadImageDataUri(url)]),
  )
  return Object.fromEntries(entries.filter(([, dataUri]) => dataUri))
}

function setsTable(exercise, t) {
  const fields = columnsFor(exercise.tracking_type)
  const header = [
    { text: t('programs.setNumber'), bold: true, fontSize: 8 },
    ...fields.map((field) => ({ text: t(`programs.fields.${field}`), bold: true, fontSize: 8 })),
  ]
  const rows = exercise.sets.map((set) => [
    String(set.set_number),
    ...fields.map((field) => formatValue(set, field, t)),
  ])
  return {
    table: { headerRows: 1, widths: [20, ...fields.map(() => '*')], body: [header, ...rows] },
    layout: 'lightHorizontalLines',
    margin: [0, 4, 0, 0],
  }
}

function exerciseBody(exercise, { t, trackingTypeNames }) {
  const body = [{ text: exercise.name, bold: true, fontSize: 11 }]
  const tags = [exercise.equipment, trackingTypeNames?.[exercise.tracking_type]]
    .filter(Boolean)
    .join(' · ')
  if (tags) {
    body.push({ text: tags, fontSize: 8, color: MUTED_COLOR, margin: [0, 2, 0, 4] })
  }
  if (exercise.sets?.length) {
    body.push(setsTable(exercise, t))
  } else {
    body.push({ text: t('programs.noSets'), fontSize: 9, color: MUTED_COLOR })
  }
  return body
}

function exerciseBlock(exercise, options) {
  const dataUri = options.images?.[mediaUrl(exercise)]
  if (!dataUri) {
    return [{ stack: exerciseBody(exercise, options), margin: [0, 10, 0, 10] }]
  }
  return [
    {
      columns: [
        { image: dataUri, fit: [IMAGE_SIZE, IMAGE_SIZE], width: IMAGE_SIZE },
        { stack: exerciseBody(exercise, options), width: '*' },
      ],
      columnGap: 12,
      margin: [0, 10, 0, 10],
      // картинка и таблица не должны разъезжаться по страницам;
      // при очень длинной таблице подходов блок делаем разрывным
      unbreakable: (exercise.sets?.length ?? 0) <= 20,
    },
  ]
}

function dayBlock(day, options) {
  const block = [
    { text: day.name, bold: true, fontSize: 14, margin: [0, 16, 0, 2] },
    {
      text: options.t('programs.exercisesCount', day.exercises.length),
      fontSize: 9,
      color: MUTED_COLOR,
      margin: [0, 0, 0, 4],
    },
  ]
  if (day.exercises.length === 0) {
    block.push({
      text: options.t('programs.emptyDay'),
      fontSize: 9,
      color: MUTED_COLOR,
      margin: [0, 0, 0, 8],
    })
    return block
  }
  for (const exercise of day.exercises) {
    block.push(...exerciseBlock(exercise, options))
  }
  return block
}

function metaLine(detail, { t, locale }) {
  const parts = []
  if (detail.level) parts.push(t(`profile.level_${detail.level}`))
  if (detail.author_name) parts.push(detail.author_name)
  if (detail.updated_at) {
    const date = new Date(detail.updated_at).toLocaleDateString(locale)
    parts.push(t('programs.pdfUpdatedAt', { date }))
  }
  return parts.join(' · ')
}

function pdfFileName(detail) {
  const base = String(detail?.name ?? '')
    .replace(/[\\/:*?"<>|\r\n\t]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 80)
  return `${base || `program-${detail?.id ?? 'export'}`}.pdf`
}

// detail — объект Detail из GET /programs/:id,
// t — функция vue-i18n, locale — текущая локаль ('ru' | 'en')
export async function exportProgramPdf(detail, { t, locale, trackingTypeNames }) {
  const [{ default: pdfMake }, { default: vfs }, images] = await Promise.all([
    import('pdfmake/build/pdfmake.js'),
    import('pdfmake/build/vfs_fonts.js'),
    loadImages(detail),
  ])
  pdfMake.addVirtualFileSystem(vfs)

  const content = [
    { text: detail.name, bold: true, fontSize: 20, margin: [0, 0, 0, 6] },
  ]
  const meta = metaLine(detail, { t, locale })
  if (meta) {
    content.push({ text: meta, fontSize: 9, color: MUTED_COLOR, margin: [0, 0, 0, 10] })
  }
  if (detail.description) {
    content.push({ text: detail.description, fontSize: 10, margin: [0, 0, 0, 10] })
  }
  for (const day of detail.days ?? []) {
    content.push(...dayBlock(day, { t, trackingTypeNames, images }))
  }

  const docDefinition = {
    defaultStyle: { font: 'Roboto', fontSize: 10, lineHeight: 1.3 },
    content,
    footer: (currentPage, pageCount) => ({
      text: `${currentPage} / ${pageCount}`,
      alignment: 'center',
      fontSize: 8,
      color: MUTED_COLOR,
      margin: [0, 10, 0, 0],
    }),
  }

  await pdfMake.createPdf(docDefinition).download(pdfFileName(detail))
}
