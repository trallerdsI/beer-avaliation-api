import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/beer.dart';
import '../repositories/i_beer_repository.dart';
import '../../../../core/usecase/usecase.dart';

class SearchBeersParams {
  final String query;
  final String? styleFilter;
  const SearchBeersParams({required this.query, this.styleFilter});
}

class SearchBeersUseCase implements UseCase<SearchBeersParams, List<Beer>> {
  SearchBeersUseCase({required this.beerRepository});

  final IBeerRepository beerRepository;

  @override
  Future<Either<Failure, List<Beer>>> call(SearchBeersParams params) async {
    return beerRepository.searchBeers(query: params.query, styleFilter: params.styleFilter);
  }
}
