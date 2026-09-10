import clsx from 'clsx'
import { MapPin, Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { errorMessage } from '../../api/client'
import { formatDistance, haversineMeters, isValidLngLat, type LngLat } from '../../utils/geo'
import Button from '../ui/Button'
import Input from '../ui/Input'
import type { AMapNS, AMapPoi } from './amap-types'
import MapView from './MapView'
import { DEFAULT_CENTER } from './marker-style'
import type { MapMarker } from './types'
import { useMapEngine } from './useMapEngine'

export interface PickedPoint {
  lng: number
  lat: number
  address?: string
}

export interface MapPickerProps {
  value: PickedPoint | null
  onChange: (point: PickedPoint | null) => void
  /** 参照点（如出发地）：显示直线距离，并作为示意图的视野参照 */
  origin?: LngLat | null
  originLabel?: string
  /** 无 value / origin 时的初始中心 */
  center?: LngLat
  /** 选点标记的文字 */
  pointLabel?: string
  height?: number | string
  disabled?: boolean
  invalid?: boolean
  id?: string
  className?: string
  /** 有 Key 时的搜索框占位 */
  placeholder?: string
}

interface SearchHit {
  key: string
  name: string
  address: string
  lnglat: LngLat
}

function poiAddress(p: AMapPoi): string {
  const addr = Array.isArray(p.address) ? p.address.join('') : (p.address ?? '')
  return [p.pname, p.cityname, p.adname, addr].filter((s): s is string => Boolean(s) && s !== '[]').join('')
}

/** 有高德 Key：关键字搜索（AMap.PlaceSearch）+ 地图点击（逆地理补地址） */
function AMapSearch({ onPick, disabled, placeholder }: { onPick: (p: PickedPoint) => void; disabled?: boolean; placeholder: string }) {
  const { loadAMap } = useMapEngine()
  const [keyword, setKeyword] = useState('')
  const [hits, setHits] = useState<SearchHit[]>([])
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const seq = useRef(0)

  const search = async () => {
    const kw = keyword.trim()
    if (!kw) return
    const my = ++seq.current
    setSearching(true)
    setError(null)
    try {
      const ns: AMapNS = await loadAMap()
      const ps = new ns.PlaceSearch({ pageSize: 8, pageIndex: 1, extensions: 'base' })
      ps.search(kw, (status, result) => {
        if (my !== seq.current) return
        setSearching(false)
        if (status === 'complete' && typeof result === 'object') {
          const pois = result.poiList?.pois ?? []
          setHits(
            pois.flatMap((p, i): SearchHit[] => {
              const loc = p.location
              if (!loc || !isValidLngLat([loc.lng, loc.lat])) return []
              return [{ key: p.id ?? String(i), name: p.name, address: poiAddress(p), lnglat: [loc.lng, loc.lat] }]
            }),
          )
        } else if (status === 'no_data') {
          setHits([])
        } else {
          setHits([])
          setError(typeof result === 'string' ? result : '搜索失败')
        }
      })
    } catch (e) {
      if (my !== seq.current) return
      setSearching(false)
      setError(errorMessage(e, '地图服务不可用'))
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <Input
          icon={Search}
          value={keyword}
          disabled={disabled}
          onChange={(e) => setKeyword(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              void search()
            }
          }}
          placeholder={placeholder}
          aria-label="地点搜索"
        />
        <Button variant="secondary" onClick={() => void search()} loading={searching} disabled={disabled || !keyword.trim()}>
          搜索
        </Button>
      </div>
      {error && <div className="text-xs text-danger-200">{error}</div>}
      {hits.length > 0 && (
        <ul className="max-h-40 divide-y divide-line-soft overflow-y-auto rounded-lg border border-line-strong bg-surface-2 text-sm">
          {hits.map((h) => (
            <li key={h.key}>
              <button
                type="button"
                disabled={disabled}
                onClick={() => {
                  onPick({ lng: h.lnglat[0], lat: h.lnglat[1], address: h.address ? `${h.name}（${h.address}）` : h.name })
                  setHits([])
                }}
                className="flex w-full items-start gap-2 px-3 py-2 text-left hover:bg-surface-3"
              >
                <MapPin size={14} className="mt-0.5 shrink-0 text-brand-600" />
                <span className="min-w-0">
                  <span className="block truncate font-medium text-ink-strong">{h.name}</span>
                  {h.address && <span className="block truncate text-xs text-ink-faint">{h.address}</span>}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

/** 无 Key：经纬度手动输入 */
function CoordInputs({ value, onChange, disabled, invalid, id }: { value: PickedPoint | null; onChange: (p: PickedPoint | null) => void; disabled?: boolean; invalid?: boolean; id?: string }) {
  const [lng, setLng] = useState(value ? String(value.lng) : '')
  const [lat, setLat] = useState(value ? String(value.lat) : '')
  // 外部值变化（地图点击 / 清空）时同步输入框
  const key = value ? `${value.lng},${value.lat}` : ''
  const [syncedKey, setSyncedKey] = useState(key)
  if (key !== syncedKey) {
    setSyncedKey(key)
    setLng(value ? String(value.lng) : '')
    setLat(value ? String(value.lat) : '')
  }

  const commit = (nextLng: string, nextLat: string) => {
    const a = Number(nextLng)
    const b = Number(nextLat)
    if (nextLng.trim() === '' && nextLat.trim() === '') {
      if (value) onChange(null)
      return
    }
    if (isValidLngLat([a, b])) {
      if (!value || value.lng !== a || value.lat !== b) onChange({ lng: a, lat: b })
    }
  }

  return (
    <div className="grid grid-cols-2 gap-2">
      <Input
        id={id}
        type="number"
        step="0.000001"
        min={-180}
        max={180}
        inputMode="decimal"
        placeholder="经度，如 116.397"
        aria-label="经度"
        value={lng}
        disabled={disabled}
        invalid={invalid}
        onChange={(e) => {
          setLng(e.target.value)
          commit(e.target.value, lat)
        }}
      />
      <Input
        type="number"
        step="0.000001"
        min={-90}
        max={90}
        inputMode="decimal"
        placeholder="纬度，如 39.908"
        aria-label="纬度"
        value={lat}
        disabled={disabled}
        invalid={invalid}
        onChange={(e) => {
          setLat(e.target.value)
          commit(lng, e.target.value)
        }}
      />
    </div>
  )
}

/**
 * 地图选点：
 * - 有 Key：高德 PlaceSearch 关键字搜索 + 地图点击（Geocoder 逆地理补地址）
 * - 无 Key：经纬度输入 + 在示意图上点击（以参照点 / 初始中心为视野）
 * 输出 `{lng, lat, address?}`；有参照点时显示与之的直线距离。
 */
export default function MapPicker({ value, onChange, origin, originLabel = '起点', center, pointLabel = '选点', height = 280, disabled, invalid, id, className, placeholder = '搜索地点，如 xx 大厦' }: MapPickerProps) {
  const { engine, loadAMap } = useMapEngine()
  const o = origin && isValidLngLat(origin) ? origin : null
  // 初始中心只在首次挂载时确定，之后选点不会拉回视野
  const [initialCenter] = useState<LngLat>(() => (value && isValidLngLat([value.lng, value.lat]) ? [value.lng, value.lat] : (o ?? center ?? DEFAULT_CENTER)))
  const geocodeSeq = useRef(0)

  const pick = useCallback(
    (lnglat: LngLat) => {
      if (disabled) return
      const my = ++geocodeSeq.current
      onChange({ lng: lnglat[0], lat: lnglat[1] })
      if (engine !== 'amap') return
      // 逆地理编码补地址（异步，若期间已改选则忽略）
      loadAMap()
        .then((ns) => {
          const geocoder = new ns.Geocoder({ radius: 500 })
          geocoder.getAddress(lnglat, (status, result) => {
            if (my !== geocodeSeq.current) return
            if (status === 'complete' && typeof result === 'object' && result.regeocode?.formattedAddress) {
              onChange({ lng: lnglat[0], lat: lnglat[1], address: result.regeocode.formattedAddress })
            }
          })
        })
        .catch(() => undefined)
    },
    [disabled, engine, loadAMap, onChange],
  )

  useEffect(() => {
    // 清空时作废进行中的逆地理回调
    if (!value) geocodeSeq.current += 1
  }, [value])

  const markers = useMemo<MapMarker[]>(() => {
    const out: MapMarker[] = []
    if (o) out.push({ id: 'origin', lnglat: o, icon: 'start', label: originLabel, zIndex: 2 })
    if (value && isValidLngLat([value.lng, value.lat])) out.push({ id: 'picked', lnglat: [value.lng, value.lat], icon: 'pin', label: pointLabel, zIndex: 5 })
    return out
  }, [o, value, originLabel, pointLabel])

  const distance = o && value ? haversineMeters(o, [value.lng, value.lat]) : null

  return (
    <div className={clsx('space-y-2', className)}>
      {engine === 'amap' ? (
        <AMapSearch onPick={(p) => !disabled && onChange(p)} disabled={disabled} placeholder={placeholder} />
      ) : (
        <CoordInputs id={id} value={value} onChange={onChange} disabled={disabled} invalid={invalid} />
      )}
      <MapView
        height={height}
        center={initialCenter}
        zoom={14}
        spanDeg={0.03}
        markers={markers}
        fitKey={o ? o.join(',') : 'none'}
        onClick={pick}
        hint="点击地图选点"
        className={clsx(invalid && 'border-danger-400', disabled && 'opacity-70')}
      />
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-muted">
        {value ? (
          <>
            <span className="inline-flex items-center gap-1 text-ink">
              <MapPin size={12} className="text-brand-600" />
              <span className="font-mono">
                {value.lng.toFixed(6)}, {value.lat.toFixed(6)}
              </span>
            </span>
            {value.address && <span className="truncate">{value.address}</span>}
            {distance !== null && (
              <span>
                距{originLabel} <span className="font-medium text-ink">{formatDistance(distance)}</span>
              </span>
            )}
            {!disabled && (
              <button type="button" onClick={() => onChange(null)} className="inline-flex items-center gap-0.5 text-ink-faint hover:text-danger-200">
                <X size={12} />
                清除
              </button>
            )}
          </>
        ) : (
          <span>{engine === 'amap' ? '搜索地点或点击地图选点' : '输入经纬度或点击示意图选点'}</span>
        )}
      </div>
    </div>
  )
}
