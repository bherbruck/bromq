import {
  type ColumnDef,
  type ColumnFiltersState,
  type OnChangeFn,
  type PaginationState,
  type SortingState,
  type VisibilityState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useCallback, useState } from 'react'

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table'
import { DataTablePagination } from './data-table-pagination'
import { DataTableToolbar } from './data-table-toolbar'

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[]
  data: TData[]
  searchColumn?: string
  searchPlaceholder?: string
  getRowClassName?: (row: TData) => string

  // Server-side pagination props (fully controlled)
  pageCount?: number
  pagination?: PaginationState // Controlled pagination state
  sorting?: SortingState // Controlled sorting state
  columnFilters?: ColumnFiltersState // Controlled filter state

  manualPagination?: boolean
  manualSorting?: boolean
  manualFiltering?: boolean

  onPaginationChange?: (updater: (old: PaginationState) => PaginationState) => void
  onSortingChange?: (updater: (old: SortingState) => SortingState) => void
  onGlobalFilterChange?: (filter: string) => void
}

export function DataTable<TData, TValue>({
  columns,
  data,
  searchColumn,
  searchPlaceholder,
  getRowClassName,
  pageCount,
  pagination: controlledPagination,
  sorting: controlledSorting,
  columnFilters: controlledColumnFilters,
  manualPagination = false,
  manualSorting = false,
  manualFiltering = false,
  onPaginationChange,
  onSortingChange: onSortingChangeExternal,
  onGlobalFilterChange,
}: DataTableProps<TData, TValue>) {
  // Use controlled state if provided, otherwise use internal state
  const [internalSorting, setInternalSorting] = useState<SortingState>([])
  const [internalColumnFilters, setInternalColumnFilters] = useState<ColumnFiltersState>([])
  const [internalPagination, setInternalPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 25,
  })

  const sorting = controlledSorting ?? internalSorting
  const columnFilters = controlledColumnFilters ?? internalColumnFilters
  const pagination = controlledPagination ?? internalPagination

  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [rowSelection, setRowSelection] = useState({})

  // Wrapper functions that update state AND notify parent
  const handlePaginationChange: OnChangeFn<PaginationState> = useCallback(
    (updaterOrValue) => {
      const updater = typeof updaterOrValue === 'function' ? updaterOrValue : () => updaterOrValue

      if (onPaginationChange) {
        // If controlled, just notify parent
        onPaginationChange(updater)
      } else {
        // If uncontrolled, update internal state
        setInternalPagination(updater)
      }
    },
    [onPaginationChange],
  )

  const handleSortingChange: OnChangeFn<SortingState> = useCallback(
    (updaterOrValue) => {
      const updater = typeof updaterOrValue === 'function' ? updaterOrValue : () => updaterOrValue

      if (onSortingChangeExternal) {
        // If controlled, just notify parent
        onSortingChangeExternal(updater)
      } else {
        // If uncontrolled, update internal state
        setInternalSorting(updater)
      }
    },
    [onSortingChangeExternal],
  )

  const handleColumnFiltersChange: OnChangeFn<ColumnFiltersState> = useCallback(
    (updaterOrValue) => {
      const updater = typeof updaterOrValue === 'function' ? updaterOrValue : () => updaterOrValue

      if (onGlobalFilterChange) {
        // Extract search value and notify parent
        const newState = updater(controlledColumnFilters ?? internalColumnFilters)
        const filter = newState.find((f) => f.id === searchColumn)
        onGlobalFilterChange(filter?.value as string || '')
      } else {
        // If uncontrolled, update internal state
        setInternalColumnFilters(updater)
      }
    },
    [onGlobalFilterChange, searchColumn, controlledColumnFilters, internalColumnFilters],
  )

  const table = useReactTable({
    data,
    columns,
    pageCount: pageCount ?? -1,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: manualPagination ? undefined : getPaginationRowModel(),
    onSortingChange: handleSortingChange,
    getSortedRowModel: manualSorting ? undefined : getSortedRowModel(),
    onColumnFiltersChange: handleColumnFiltersChange,
    getFilteredRowModel: manualFiltering ? undefined : getFilteredRowModel(),
    onColumnVisibilityChange: setColumnVisibility,
    onRowSelectionChange: setRowSelection,
    onPaginationChange: handlePaginationChange,
    manualPagination,
    manualSorting,
    manualFiltering,
    state: {
      sorting,
      columnFilters,
      columnVisibility,
      rowSelection,
      pagination,
    },
  })

  return (
    <div className="space-y-4">
      <DataTableToolbar
        table={table}
        searchColumn={searchColumn}
        searchPlaceholder={searchPlaceholder}
      />
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead key={header.id}>
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className={getRowClassName?.(row.original)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center">
                  No results.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <DataTablePagination table={table} />
    </div>
  )
}
