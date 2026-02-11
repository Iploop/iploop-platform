'use client'

import { memo, useMemo } from 'react'
import {
  ComposableMap,
  Geographies,
  Geography,
  Marker,
  ZoomableGroup,
} from 'react-simple-maps'

const GEO_URL = 'https://cdn.jsdelivr.net/npm/world-atlas@2/countries-110m.json'

// Country code to approximate lat/lng centroids
const COUNTRY_COORDS: Record<string, [number, number]> = {
  US: [-98, 39], CA: [-106, 56], MX: [-102, 23], BR: [-51, -10], AR: [-64, -34],
  GB: [-2, 54], DE: [10, 51], FR: [2, 46], ES: [-4, 40], IT: [12, 42],
  NL: [5, 52], BE: [4, 50], CH: [8, 47], AT: [14, 47], PL: [20, 52],
  SE: [15, 62], NO: [8, 62], DK: [10, 56], FI: [26, 64], IE: [-8, 53],
  PT: [-8, 39], CZ: [15, 50], RO: [25, 46], BG: [25, 43], HR: [16, 45],
  GR: [22, 39], HU: [19, 47], SK: [19, 48], RS: [21, 44], UA: [31, 49],
  RU: [100, 60], TR: [35, 39], IL: [35, 31], AE: [54, 24], SA: [45, 24],
  IN: [79, 21], JP: [138, 36], KR: [128, 36], CN: [104, 35], TW: [121, 24],
  AU: [134, -25], NZ: [174, -41], ZA: [25, -29], NG: [8, 10], KE: [38, 0],
  EG: [30, 27], MA: [-5, 32], GH: [-2, 8], TH: [100, 15], PH: [122, 12],
  ID: [117, -2], MY: [110, 4], SG: [104, 1], VN: [106, 16], PK: [69, 30],
  BD: [90, 24], CO: [-74, 4], CL: [-71, -35], PE: [-76, -10], VE: [-66, 7],
  EC: [-78, -2], BB: [-59, 13], TT: [-61, 10], JM: [-77, 18], PR: [-66, 18],
  DO: [-70, 19], PA: [-80, 9], CR: [-84, 10], GT: [-90, 15], HN: [-87, 14],
  NI: [-85, 13], SV: [-89, 14], BZ: [-88, 17], BS: [-77, 25], KY: [-81, 19],
  CU: [-80, 22], HT: [-72, 19], BM: [-64, 32], AW: [-70, 12], CW: [-69, 12],
  DZ: [3, 28], LY: [17, 27], TN: [9, 34], ET: [40, 9], TZ: [35, -6],
  UG: [32, 1], MZ: [35, -18], ZW: [30, -20], BW: [24, -22], NA: [18, -22],
  AO: [18, -12], CM: [12, 6], CI: [-5, 7], SN: [-14, 14], GA: [12, -1],
  CG: [15, -1], CD: [24, -3], MG: [47, -20], MU: [57, -20], SC: [55, -4],
  FJ: [178, -18], PG: [147, -6], WS: [-172, -14], TO: [-175, -21],
  LC: [-61, 14], GD: [-61, 12], AG: [-61, 17], KN: [-62, 17], DM: [-61, 15],
  VC: [-61, 13], AI: [-63, 18], GI: [-5, 36], IM: [-4, 54], JE: [-2, 49],
  GG: [-2, 49], LI: [9, 47], LU: [6, 49], MT: [14, 36], IS: [-19, 65],
  GE: [44, 42], AM: [45, 40], AZ: [48, 40], KZ: [67, 48], UZ: [64, 41],
  KG: [75, 41], TJ: [69, 39], TM: [59, 39], MN: [104, 47], KW: [48, 29],
  QA: [51, 25], BH: [50, 26], OM: [56, 21], JO: [36, 31], LB: [36, 34],
  IQ: [44, 33], IR: [53, 32], AF: [67, 33], LK: [81, 7], NP: [84, 28],
  MM: [96, 20], KH: [105, 13], LA: [103, 18], BN: [115, 5],
  AL: [20, 41], BA: [18, 44], ME: [19, 42], MK: [22, 41], SI: [15, 46],
  EE: [25, 59], LV: [24, 57], LT: [24, 56], BY: [28, 53], MD: [29, 47],
  XK: [21, 43], CY: [33, 35],
  GY: [-59, 5], SR: [-56, 4],
}

interface WorldMapProps {
  countryData: Record<string, number>
}

function WorldMapComponent({ countryData }: WorldMapProps) {
  const markers = useMemo(() => {
    return Object.entries(countryData)
      .filter(([code]) => code && code !== '' && COUNTRY_COORDS[code])
      .map(([code, count]) => ({
        code,
        count,
        coordinates: COUNTRY_COORDS[code] as [number, number],
      }))
  }, [countryData])

  const maxCount = Math.max(...Object.values(countryData), 1)

  // Build a set of country codes that have nodes for highlighting
  const activeCountries = useMemo(() => {
    return new Set(Object.keys(countryData).filter(c => c && countryData[c] > 0))
  }, [countryData])

  // Map ISO 3166-1 alpha-2 to alpha-3 for matching with topojson
  const alpha2to3: Record<string, string> = {
    US:'USA',CA:'CAN',MX:'MEX',BR:'BRA',AR:'ARG',GB:'GBR',DE:'DEU',FR:'FRA',
    ES:'ESP',IT:'ITA',NL:'NLD',BE:'BEL',CH:'CHE',AT:'AUT',PL:'POL',SE:'SWE',
    NO:'NOR',DK:'DNK',FI:'FIN',IE:'IRL',PT:'PRT',CZ:'CZE',RO:'ROU',BG:'BGR',
    HR:'HRV',GR:'GRC',HU:'HUN',SK:'SVK',RS:'SRB',UA:'UKR',RU:'RUS',TR:'TUR',
    IL:'ISR',AE:'ARE',SA:'SAU',IN:'IND',JP:'JPN',KR:'KOR',CN:'CHN',TW:'TWN',
    AU:'AUS',NZ:'NZL',ZA:'ZAF',NG:'NGA',KE:'KEN',EG:'EGY',MA:'MAR',GH:'GHA',
    TH:'THA',PH:'PHL',ID:'IDN',MY:'MYS',SG:'SGP',VN:'VNM',PK:'PAK',BD:'BGD',
    CO:'COL',CL:'CHL',PE:'PER',VE:'VEN',EC:'ECU',BB:'BRB',TT:'TTO',JM:'JAM',
    PR:'PRI',DO:'DOM',PA:'PAN',CR:'CRI',GT:'GTM',HN:'HND',DZ:'DZA',LY:'LBY',
    TN:'TUN',ET:'ETH',TZ:'TZA',UG:'UGA',AL:'ALB',BA:'BIH',EE:'EST',LV:'LVA',
    LT:'LTU',BY:'BLR',MD:'MDA',CY:'CYP',GE:'GEO',AZ:'AZE',KZ:'KAZ',KW:'KWT',
    QA:'QAT',OM:'OMN',JO:'JOR',LB:'LBN',IQ:'IRQ',IR:'IRN',AF:'AFG',LK:'LKA',
    NP:'NPL',KH:'KHM',MM:'MMR',MK:'MKD',SI:'SVN',ME:'MNE',XK:'XKX',BS:'BHS',
  }

  const activeAlpha3 = useMemo(() => {
    const s = new Set<string>()
    activeCountries.forEach(c => { if (alpha2to3[c]) s.add(alpha2to3[c]) })
    return s
  }, [activeCountries])

  return (
    <div className="w-full" style={{ aspectRatio: '2/1' }}>
      <ComposableMap
        projectionConfig={{
          rotate: [-10, 0, 0],
          scale: 147,
        }}
        style={{ width: '100%', height: '100%' }}
      >
        <ZoomableGroup center={[0, 20]} zoom={1}>
          {/* Ocean / background gradient effect */}
          <rect x={-500} y={-300} width={1500} height={700} fill="#0a0f1e" rx={0} />
          <Geographies geography={GEO_URL}>
            {({ geographies }) =>
              geographies.map((geo) => {
                const iso3 = geo.properties?.ISO_A3 || geo.properties?.iso_a3 || ''
                const isActive = activeAlpha3.has(iso3)
                return (
                  <Geography
                    key={geo.rpikey}
                    geography={geo}
                    fill={isActive ? '#1e3a5f' : '#111827'}
                    stroke="#1e3a5f"
                    strokeWidth={0.5}
                    style={{
                      default: { outline: 'none' },
                      hover: { fill: isActive ? '#234b78' : '#1a2332', outline: 'none' },
                      pressed: { outline: 'none' },
                    }}
                  />
                )
              })
            }
          </Geographies>
          {/* Grid lines for effect */}
          {[-60, -30, 0, 30, 60].map(lat => (
            <line
              key={`lat-${lat}`}
              x1={0} y1={0} x2={0} y2={0}
              stroke="#1e293b"
              strokeWidth={0.2}
              strokeDasharray="2,4"
            />
          ))}
          {markers.map(({ code, count, coordinates }) => {
            const ratio = count / maxCount
            const radius = Math.max(4, Math.min(20, 4 + ratio * 16))
            const opacity = Math.max(0.5, Math.min(1, 0.5 + ratio * 0.5))
            const isTop = count >= maxCount * 0.05
            return (
              <Marker key={code} coordinates={coordinates}>
                {/* Outer glow */}
                <circle
                  r={radius + 6}
                  fill="#3b82f6"
                  fillOpacity={opacity * 0.08}
                />
                {/* Middle ring */}
                <circle
                  r={radius + 3}
                  fill="#3b82f6"
                  fillOpacity={opacity * 0.15}
                />
                {/* Main dot */}
                <circle
                  r={radius}
                  fill={`hsl(${210 + ratio * 20}, ${70 + ratio * 20}%, ${50 + ratio * 15}%)`}
                  fillOpacity={opacity}
                  stroke="#93c5fd"
                  strokeWidth={ratio > 0.3 ? 1 : 0.5}
                  strokeOpacity={0.6}
                />
                {/* Inner highlight */}
                <circle
                  r={radius * 0.4}
                  fill="#93c5fd"
                  fillOpacity={0.3}
                />
                {isTop && (
                  <text
                    textAnchor="middle"
                    y={radius + 14}
                    style={{
                      fontSize: '9px',
                      fill: '#e2e8f0',
                      fontWeight: 600,
                      textShadow: '0 0 4px rgba(0,0,0,0.8)',
                    }}
                  >
                    {code} {count.toLocaleString()}
                  </text>
                )}
              </Marker>
            )
          })}
        </ZoomableGroup>
      </ComposableMap>
    </div>
  )
}

export const WorldMap = memo(WorldMapComponent)
